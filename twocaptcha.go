package solver

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	connect "bitbucket.org/babylonaio/pkg/http"
)

// ApiURL is the url of the 2captcha API endpoint
var ApiURL = "https://2captcha.com/in.php"

// ResultURL is the url of the 2captcha result API endpoint
var ResultURL = "https://2captcha.com/res.php"

// TwoCaptchaClient is an interface to https://2captcha.com/ API.
type TwoCaptchaClient struct {
	// ApiKey is the API key for the 2captcha.com API.
	// Valid key is required by all the functions of this library
	// See more details on https://2captcha.com/2captcha-api#solving_captchas
	ApiKey string
}

// New creates a TwoCaptchaClient instance
func NewTwoCaptcha(apiKey string) *TwoCaptchaClient {
	return &TwoCaptchaClient{
		ApiKey: apiKey,
	}
}

// SolveCaptcha performs a normal captcha solving request to 2captcha.com
// and returns with the solved captcha if the request was successful.
// Valid ApiKey is required.
// See more details on https://2captcha.com/2captcha-api#solving_normal_captcha
//func (c *TwoCaptchaClient) SolveCaptcha(url string) (string, error) {
//
//	req, _ := http.NewRequest("GET", url, nil)
//	resp, err := connect.DefaultDo(req)
//	if err != nil || resp.StatusCode != 200 {
//		return "", err
//	}
//	defer resp.Body.Close()
//
//	body, err := ioutil.ReadAll(resp.Body)
//	if err != nil {
//		return "", err
//	}
//
//	strBase64 := base64.StdEncoding.EncodeToString(body)
//
//	captchaId, err := c.apiRequest(nil,
//		ApiURL,
//		map[string]string{
//			"method":   "base64",
//			"body":     strBase64,
//			"phrase":   "1",
//			"regsense": "1",
//		},
//		0,
//		3,
//	)
//
//	if err != nil {
//		return "", err
//	}
//
//	return c.apiRequest(nil,
//		ResultURL,
//		map[string]string{
//			"id":     captchaId,
//			"action": "get",
//		},
//		5,
//		20,
//	)
//}

// SolveRecaptchaV2 performs a recaptcha v2 solving request to 2captcha.com
// and returns with the solved captcha if the request was successful.
// Valid ApiKey is required.
// See more details on https://2captcha.com/2captcha-api#solving_recaptchav2_new
func (c *TwoCaptchaClient) SolveRecaptchaV2(ctx context.Context, siteURL, recaptchaKey string) (string, error) {
	captchaId, err := c.apiRequest(ctx,
		ApiURL,
		map[string]string{
			"googlekey": recaptchaKey,
			"pageurl":   siteURL,
			"method":    "userrecaptcha",
		},
		0,
		3,
	)

	log.Println(captchaId, err)
	if err != nil {
		return "", err
	}

	return c.apiRequest(ctx,
		ResultURL,
		map[string]string{
			"googlekey": recaptchaKey,
			"pageurl":   siteURL,
			"method":    "userrecaptcha",
			"id":        captchaId,
			"action":    "get",
		},
		3,
		20,
	)
}

func (c *TwoCaptchaClient) apiRequest(ctx context.Context, URL string, params map[string]string, delay time.Duration, retries int) (string, error) {
	if retries <= 0 {
		return "", errors.New("Maximum retries exceeded")
	}
	time.Sleep(delay * time.Second)
	form := url.Values{}
	form.Add("key", c.ApiKey)
	for k, v := range params {
		form.Add(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", URL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	resp, err := connect.DefaultDo(req)
	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	log.Println(string(body), err)
	resp.Body.Close()
	if strings.Contains(string(body), "CAPCHA_NOT_READY") {
		return c.apiRequest(ctx, URL, params, delay, retries-1)
	}
	if !strings.Contains(string(body), "OK|") {
		return "", errors.New("Invalid respponse from 2captcha: " + string(body))
	}
	return string(body[3:]), nil
}
