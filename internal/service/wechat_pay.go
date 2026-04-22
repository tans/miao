package service

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// WechatPayService 微信支付服务
type WechatPayService struct {
	AppID      string
	MchID      string
	SerialNo   string
	PrivateKey *rsa.PrivateKey
	ApiV3Key   string
}

// WechatPayConfig 微信支付配置
type WechatPayConfig struct {
	AppID         string
	MchID         string
	SerialNo      string
	PrivateKeyPath string
	ApiV3Key      string
}

// NewWechatPayService 创建微信支付服务
func NewWechatPayService(cfg WechatPayConfig) (*WechatPayService, error) {
	keyData, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥失败: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("解析私钥失败: PEM decode failed")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return &WechatPayService{
		AppID:      cfg.AppID,
		MchID:      cfg.MchID,
		SerialNo:   cfg.SerialNo,
		PrivateKey: privateKey,
		ApiV3Key:   cfg.ApiV3Key,
	}, nil
}

// UnifiedOrderRequest 统一下单请求
type UnifiedOrderRequest struct {
	Description string `json:"description"`
	OutTradeNo  string `json:"out_trade_no"`
	Amount      struct {
		Total    int    `json:"total"` // 金额，单位分
		Currency string `json:"currency"`
	} `json:"amount"`
	NotifyURL string `json:"notify_url"`
}

// UnifiedOrderResponse 统一下单响应
type UnifiedOrderResponse struct {
	Code          string `json:"code"`
	Message       string `json:"message"`
	CodeUrl       string `json:"code_url"`
	AppID         string `json:"appid"`
	MchID         string `json:"mchid"`
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeState    string `json:"trade_state"`
	TradeStateDesc string `json:"trade_state_desc"`
}

// UnifiedOrder 统一下单
func (s *WechatPayService) UnifiedOrder(req *UnifiedOrderRequest) (*UnifiedOrderResponse, error) {
	url := "https://api.mch.weixin.qq.com/v3/pay/transactions/native"

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	token, err := s.makeToken("POST", "/v3/pay/transactions/native", string(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result UnifiedOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %s", string(respBody))
	}

	return &result, nil
}

// QueryOrder 查询订单
func (s *WechatPayService) QueryOrder(outTradeNo string) (*UnifiedOrderResponse, error) {
	url := fmt.Sprintf("https://api.mch.weixin.qq.com/v3/pay/transactions/out-trade-no/%s", outTradeNo)

	httpReq, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	token, err := s.makeToken("GET", "/v3/pay/transactions/out-trade-no/"+outTradeNo, "")
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", token)
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result UnifiedOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %s", string(respBody))
	}

	return &result, nil
}

// makeToken 生成签名 token
func (s *WechatPayService) makeToken(method, url, body string) (string, error) {
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonce()
	message := method + "\n" + url + "\n" + timestamp + "\n" + nonce + "\n" + body + "\n"

	h := sha256.Sum256([]byte(message))
	hashed := h[:]

	sig, err := rsa.SignPKCS1v15(nil, nil, crypto.SHA256, hashed)
	if err != nil {
		return "", err
	}
	signature := base64.StdEncoding.EncodeToString(sig)

	token := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",serial_no="%s",timestamp="%s",nonce_str="%s",signature="%s"`,
		s.MchID, s.SerialNo, timestamp, nonce, signature)
	return token, nil
}

func generateNonce() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}