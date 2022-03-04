package solver

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	connect "bitbucket.org/babylonaio/pkg/http"
	"github.com/tidwall/gjson"
)

type CapmonsterClient struct {
	ClientKey string
	Host      string
}

func InitCapmonsterClient(key string) *CapmonsterClient {
	c := &CapmonsterClient{
		ClientKey: key,
		Host:      "https://api.capmonster.cloud",
	}
	return c
}

func (c *CapmonsterClient) CreateToken(ctx context.Context, url, siteKey, proxyAddr, proxyPort, proxyLogin, proxyPass string) (string, error) {
	//	retries := 5
	//	if retries == 0 {
	//		return ""
	//	}
	taskId, err := c.createTask(ctx, url, siteKey, proxyAddr, proxyPort, proxyLogin, proxyPass)
	if err != nil {
		return "", err
	}
	if taskId == 0 {
		return "", nil
	}
	token, err := c.getTaskResult(ctx, taskId)
	if err != nil {
		return "", err
	}
	if token != "" {
		return token, nil
	}
	//	retries--
	return "", err

}

func (c *CapmonsterClient) createTask(ctx context.Context, webUrl, siteKey, proxyAddr, proxyPort, proxyLogin, proxyPass string) (float64, error) {
	u, _ := url.Parse(c.Host + "/createTask")
	m := map[string]interface{}{
		"clientKey": c.ClientKey,
		"task": map[string]interface{}{
			"type":          "NoCaptchaTask",
			"websiteURL":    webUrl,
			"websiteKey":    siteKey,
			"proxyType":     "https",
			"proxyAddress":  proxyAddr,
			"proxyPort":     proxyPort,
			"proxyLogin":    proxyLogin,
			"proxyPassword": proxyPass,
		},
	}

	req := (&http.Request{
		Method: "POST",
		Header: make(http.Header),
		URL:    u,
		Host:   u.Host,
	}).WithContext(ctx)
	reqBody, len, ok := connect.CreateRequestBody(m)
	if ok {
		req.Body = reqBody
		req.ContentLength = len
	}
	resp, err := connect.DefaultDo(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return gjson.Get(string(body), "taskId").Float(), nil
}

func (c *CapmonsterClient) getTaskResult(ctx context.Context, taskId float64) (string, error) {
	retries := 7
	for {
		if retries <= 0 {
			return "", nil
		}
		u, _ := url.Parse(c.Host + "/getTaskResult")
		m := map[string]interface{}{
			"clientKey": c.ClientKey,
			"taskId":    taskId,
		}
		req := (&http.Request{
			Method: "POST",
			Header: make(http.Header),
			URL:    u,
			Host:   u.Host,
		}).WithContext(ctx)
		reqBody, len, ok := connect.CreateRequestBody(m)
		if ok {
			req.Body = reqBody
			req.ContentLength = len
		}
		resp, err := connect.DefaultDo(req)
		if err != nil {
			return "", err
		}
		body, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()

		status := gjson.Get(string(body), "status").String()
		if status == "processing" {
			time.Sleep(3000 * time.Millisecond)
			retries--
			continue
		}
		return gjson.Get(string(body), "solution.gRecaptchaResponse").String(), nil
	}
}
