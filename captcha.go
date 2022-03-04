package solver

import (
	"context"
	"fmt"
	"sync"
	"time"

	d "bitbucket.org/babylonaio/pkg/datastore"
)

type Captcha interface {
	Solve(ctx context.Context) string
}

type TwoCaptcha struct {
	client *TwoCaptchaClient
}

type AntiCaptcha struct {
	client  *AntiCaptchaClient
	proxies []*d.Proxy
	index   int
	mu      sync.RWMutex
}

type Capmonster struct {
	client  *CapmonsterClient
	proxies []*d.Proxy
	index   int
	mu      sync.RWMutex
}

var datadomeURL = "https://geo.captcha-delivery.com"
var siteKey = "6LccSjEUAAAAANCPhaM2c-WiRxCZ5CzsjR_vd8uX"

func InitTwoCaptcha(key string) *TwoCaptcha {
	return &TwoCaptcha{client: NewTwoCaptcha(key)}
}

func (c *TwoCaptcha) Solve(ctx context.Context) string {
	token, err := c.client.SolveRecaptchaV2(ctx, datadomeURL, siteKey)
	if err != nil {
		fmt.Println("ERROR: ", err.Error())
		return ""
	}
	return token
}

func InitCapmonster(key string, pg string) *Capmonster {
	if d.DStore.ProxyGroups != nil && len(d.DStore.ProxyGroups) > 0 && pg != "" {
		for _, p := range d.DStore.ProxyGroups {
			if p.ID == pg {
				return &Capmonster{client: InitCapmonsterClient(key), proxies: p.Proxies}
			}
		}

	}
	return &Capmonster{client: InitCapmonsterClient(key), proxies: make([]*d.Proxy, 0)}
}

func (c *Capmonster) Solve(ctx context.Context) string {
REDO:
	var prox *d.Proxy
	if len(c.proxies) > 0 {
		c.mu.RLock()
		prox = c.proxies[c.index]
		c.mu.RUnlock()
	}
	token := ""
	if prox != nil {
		t, err := c.client.CreateToken(ctx, datadomeURL, siteKey, prox.Host, prox.Port, prox.Username, prox.Password)
		if err != nil {
			return ""
		}
		token = t
	} else {
		t, err := c.client.CreateToken(ctx, datadomeURL, siteKey, "", "", "", "")
		if err != nil {
			return ""
		}
		token = t
	}
	if token == "" && len(c.proxies) > 0 {
		c.RotateProxy()
		goto REDO
	}
	return token
}

func (c *Capmonster) RotateProxy() {
	c.mu.Lock()
	c.index = (c.index + 1) % len(c.proxies)
	c.mu.Unlock()
}

func (a *AntiCaptcha) RotateProxy() {
	a.mu.Lock()
	a.index = (a.index + 1) % len(a.proxies)
	a.mu.Unlock()
}

func InitAntiCaptcha(key, pg string) *AntiCaptcha {
	if d.DStore.ProxyGroups != nil && len(d.DStore.ProxyGroups) > 0 {
		for _, p := range d.DStore.ProxyGroups {
			if p.ID == pg {
				return &AntiCaptcha{client: &AntiCaptchaClient{APIKey: key}, proxies: p.Proxies}
			}
		}

	}
	return &AntiCaptcha{client: &AntiCaptchaClient{APIKey: key}, proxies: make([]*d.Proxy, 0)}
}
func (c *AntiCaptcha) Solve(ctx context.Context) string {
REDO:
	var prox *d.Proxy
	if len(c.proxies) > 0 {
		c.mu.RLock()
		prox = c.proxies[c.index]
		c.mu.RUnlock()
	}
	token := ""
	if prox != nil {
		t, err := c.client.SendRecaptcha(ctx, datadomeURL, siteKey, 30*time.Second, prox.Host, prox.Port, prox.Username, prox.Password)
		if err != nil {
			fmt.Println("ERROR: ", err.Error())
			return ""
		}
		token = t
	} else {
		t, err := c.client.SendRecaptcha(ctx, datadomeURL, siteKey, 30*time.Second, "", "", "", "")
		if err != nil {
			fmt.Println("ERROR: ", err.Error())
			return ""
		}
		token = t
	}
	if token == "" && len(c.proxies) > 0 {
		c.RotateProxy()
		goto REDO
	}
	return token
}