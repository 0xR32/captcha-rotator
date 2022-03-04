package solver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"time"

	connect "bitbucket.org/babylonaio/pkg/http"
)

var (
	baseURL       = &url.URL{Host: "api.anti-captcha.com", Scheme: "https", Path: "/"}
	checkInterval = 2 * time.Second
)

type AntiCaptchaClient struct {
	APIKey string
}

// Method to create the task to process the recaptcha, returns the task_id
func (c *AntiCaptchaClient) createTaskRecaptcha(ctx context.Context, websiteURL string, recaptchaKey string, proxyAddr, proxyPort, proxyLogin, proxyPass string) (float64, error) {
	// Mount the data to be sent
	body := map[string]interface{}{
		"clientKey": c.APIKey,
		"task": map[string]interface{}{
			"type":       "NoCaptchaTaskProxyless",
			"websiteURL": websiteURL,
			"websiteKey": recaptchaKey,
			//			"proxyType":     "https",
			//			"proxyAddress":  proxyAddr,
			//			"proxyPort":     proxyPort,
			//			"proxyLogin":    proxyLogin,
			//			"proxyPassword": proxyPass,
		},
	}
	// if proxyAddr != "" {
	// 	body = map[string]interface{}{
	// 		"clientKey": c.APIKey,
	// 		"task": map[string]interface{}{
	// 			"type":          "NoCaptchaTask",
	// 			"websiteURL":    websiteURL,
	// 			"websiteKey":    recaptchaKey,
	// 			"proxyType":     "https",
	// 			"proxyAddress":  proxyAddr,
	// 			"proxyPort":     proxyPort,
	// 			"proxyLogin":    proxyLogin,
	// 			"proxyPassword": proxyPass,
	// 		},
	// 	}
	// }

	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}

	// Make the request
	u := baseURL.ResolveReference(&url.URL{Path: "/createTask"})
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return 0, err
	}
	resp, err := connect.DefaultDo(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Decode response
	responseBody := make(map[string]interface{})
	json.NewDecoder(resp.Body).Decode(&responseBody)
	// TODO treat api errors and handle them properly
	if _, ok := responseBody["taskId"]; ok {
		if taskId, ok := responseBody["taskId"].(float64); ok {
			return taskId, nil
		}

		return 0, errors.New("task number of irregular format")
	}

	return 0, errors.New("task number not found in server response")
}

// Method to check the result of a given task, returns the json returned from the api
func (c *AntiCaptchaClient) getTaskResult(ctx context.Context, taskID float64) (map[string]interface{}, error) {
	// Mount the data to be sent
	body := map[string]interface{}{
		"clientKey": c.APIKey,
		"taskId":    taskID,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Make the request
	u := baseURL.ResolveReference(&url.URL{Path: "/getTaskResult"})
	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	resp, err := connect.DefaultDo(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Decode response
	responseBody := make(map[string]interface{})
	json.NewDecoder(resp.Body).Decode(&responseBody)
	return responseBody, nil
}

// SendRecaptcha Method to encapsulate the processing of the recaptcha
// Given a url and a key, it sends to the api and waits until
// the processing is complete to return the evaluated key
func (c *AntiCaptchaClient) SendRecaptcha(ctx context.Context, websiteURL string, recaptchaKey string, timeoutInterval time.Duration, proxyAddr, proxyPort, proxyLogin, proxyPass string) (string, error) {
	taskID, err := c.createTaskRecaptcha(ctx, websiteURL, recaptchaKey, proxyAddr, proxyPort, proxyLogin, proxyPass)
	if err != nil {
		return "", err
	}

	check := time.NewTicker(3 * time.Second)
	timeout := time.NewTimer(timeoutInterval)

	for {
		select {
		case <-check.C:
			response, err := c.getTaskResult(ctx, taskID)
			if err != nil {
				return "", err
			}
			if response["status"] == "ready" {
				return response["solution"].(map[string]interface{})["gRecaptchaResponse"].(string), nil
			}
			check = time.NewTicker(checkInterval)
		case <-timeout.C:
			return "", errors.New("antiCaptcha check result timeout")
		}
	}
}

// Method to create the task to process the image captcha, returns the task_id
//func (c *AntiCaptchaClient) createTaskImage(imgString string) (float64, error) {
//	// Mount the data to be sent
//	body := map[string]interface{}{
//		"clientKey": c.APIKey,
//		"task": map[string]interface{}{
//			"type": "ImageToTextTask",
//			"body": imgString,
//		},
//	}
//
//	b, err := json.Marshal(body)
//	if err != nil {
//		return 0, err
//	}
//
//	// Make the request
//	u := baseURL.ResolveReference(&url.URL{Path: "/createTask"})
//	resp, err := http.Post(u.String(), "application/json", bytes.NewBuffer(b))
//	if err != nil {
//		return 0, err
//	}
//	defer resp.Body.Close()
//
//	// Decode response
//	responseBody := make(map[string]interface{})
//	json.NewDecoder(resp.Body).Decode(&responseBody)
//	// TODO treat api errors and handle them properly
//	return responseBody["taskId"].(float64), nil
//}
//
//// SendImage Method to encapsulate the processing of the image captcha
//// Given a base64 string from the image, it sends to the api and waits until
//// the processing is complete to return the evaluated key
//func (c *AntiCaptchaClient) SendImage(imgString string) (string, error) {
//	// Create the task on anti-captcha api and get the task_id
//	taskID, err := c.createTaskImage(imgString)
//	if err != nil {
//		return "", err
//	}
//
//	// Check if the result is ready, if not loop until it is
//	response, err := c.getTaskResult(taskID)
//	if err != nil {
//		return "", err
//	}
//	for {
//		if response["status"] == "processing" {
//			time.Sleep(checkInterval)
//			response, err = c.getTaskResult(taskID)
//			if err != nil {
//				return "", err
//			}
//		} else {
//			break
//		}
//	}
//	return response["solution"].(map[string]interface{})["text"].(string), nil
//}
