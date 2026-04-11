package beian

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/imxw/icp-query-go/internal/captcha"
	"github.com/spf13/cast"
)

const (
	getCheckImageURL = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/image/getCheckImagePoint"
	checkImageURL    = "https://hlwicpfwc.miit.gov.cn/icpproject_query/api/image/checkImage"
)

// CaptchaResult holds the result of a successful captcha verification.
type CaptchaResult struct {
	PUUID   string
	Token   string
	Sign    string
	Headers map[string]string
}

func (b *Beian) checkImg(ctx context.Context, proxy string) (*CaptchaResult, error) {
	token, baseHeader, err := b.getToken(ctx, proxy)
	if err != nil {
		return nil, fmt.Errorf("获取token失败: %w", err)
	}

	// Request captcha image
	clientUID := generateClientUID()
	uidBody, err := json.Marshal(map[string]any{"clientUid": clientUID})
	if err != nil {
		return nil, fmt.Errorf("marshal captcha request: %w", err)
	}
	baseHeader["Content-Length"] = fmt.Sprintf("%d", len(uidBody))
	baseHeader["token"] = token
	baseHeader["Content-Type"] = "application/json"

	res, err := b.doPost(ctx, getCheckImageURL, uidBody, baseHeader, proxy)
	if err != nil {
		return nil, fmt.Errorf("请求验证码失败: %w", err)
	}

	var imgResult map[string]any
	if err := json.Unmarshal(res, &imgResult); err != nil {
		return nil, fmt.Errorf("解析验证码响应失败: %w", err)
	}

	params, _ := imgResult["params"].(map[string]any)
	pUUID := cast.ToString(params["uuid"])
	bigImage := cast.ToString(params["bigImage"])
	smallImage := cast.ToString(params["smallImage"])

	// Match slider
	start := time.Now()
	matchOK, offsetX, matchErr := captcha.MatchSliderOffset(smallImage, bigImage)
	if matchErr != nil {
		return nil, fmt.Errorf("滑块匹配出错: %w", matchErr)
	}
	if !matchOK {
		return nil, fmt.Errorf("滑块匹配失败")
	}
	slog.Info("slider matched", "x", offsetX, "elapsed", time.Since(start).Round(time.Millisecond))

	// Submit slider result
	checkData, err := json.Marshal(map[string]any{"key": pUUID, "value": fmt.Sprintf("%d", offsetX)})
	if err != nil {
		return nil, fmt.Errorf("marshal check request: %w", err)
	}
	baseHeader["Content-Length"] = fmt.Sprintf("%d", len(checkData))

	checkRes, err := b.doPost(ctx, checkImageURL, checkData, baseHeader, proxy)
	if err != nil {
		return nil, fmt.Errorf("提交验证码失败: %w", err)
	}

	var checkResult map[string]any
	if err := json.Unmarshal(checkRes, &checkResult); err != nil {
		return nil, fmt.Errorf("解析验证结果失败: %w", err)
	}

	slog.Info("checkImage response", "code", checkResult["code"], "msg", checkResult["msg"])

	if !cast.ToBool(checkResult["success"]) {
		return nil, fmt.Errorf("验证码识别失败")
	}

	sign := cast.ToString(checkResult["params"])
	return &CaptchaResult{
		PUUID:   pUUID,
		Token:   token,
		Sign:    sign,
		Headers: baseHeader,
	}, nil
}
