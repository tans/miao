package service

import (
	"testing"

	"github.com/tans/miao/internal/config"
	"github.com/stretchr/testify/require"
)

func TestParseMerchantAuthOCRResult(t *testing.T) {
	raw := `{"data":{"companyName":"杭州喵喵科技有限公司","creditCode":"91330100MA2GX12345","legalPerson":"张三"}}`

	result, err := parseMerchantAuthOCRResult(raw)
	require.NoError(t, err)
	require.Equal(t, "杭州喵喵科技有限公司", result.CompanyName)
	require.Equal(t, "91330100MA2GX12345", result.CreditCode)
	require.Equal(t, "张三", result.LegalPerson)
}

func TestParseMerchantAuthOCRResultFallbackToFlatFields(t *testing.T) {
	raw := `{"companyName":"杭州喵喵科技有限公司","creditCode":"91330100MA2GX12345","legalPerson":"张三"}`

	result, err := parseMerchantAuthOCRResult(raw)
	require.NoError(t, err)
	require.Equal(t, "杭州喵喵科技有限公司", result.CompanyName)
	require.Equal(t, "91330100MA2GX12345", result.CreditCode)
	require.Equal(t, "张三", result.LegalPerson)
}

func TestRecognizeBusinessLicenseDisabled(t *testing.T) {
	svc := NewMerchantAuthOCRService(&config.Config{
		OCR: config.OCRConfig{Enabled: false},
	}, nil)

	result, err := svc.RecognizeBusinessLicense(nil, "https://example.com/license.jpg")
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOCRDisabled)
}
