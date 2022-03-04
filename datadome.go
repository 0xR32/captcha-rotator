package solver

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"

	connect "bitbucket.org/babylonaio/pkg/http"
	"bitbucket.org/babylonaio/pkg/utils"
	"github.com/tidwall/gjson"
)

type CaptchaHarvester struct {
	CaptchaURL string `json:"captcha_url"`
	CID        string `json:"cid"`
	SiteKey    string `json:"site_key"`
	HSH        string `json:"hsh"`
	S          string `json:"s"`
	Domain     string `json:"domain"`
	Cookie     string `json:"cookie"`
}

func SolveDatadome(ctx context.Context, sbody, datadomeURL, datadomeCid, userAgent, pUrl, siteUrl, siteKey string, cookies []*http.Cookie) string {
	captchaUrl, cid, hsh, b := ParseBuildDatadomeURL(sbody, datadomeCid, datadomeURL, siteUrl)
	if captchaUrl == "" {
		return "rotate"
	}
	ddurl, cookies := getBuildUrl(ctx, sbody, captchaUrl, datadomeURL, datadomeCid, cid, hsh, b, userAgent, pUrl, siteUrl, siteKey, cookies)
	if ddurl == "" {
		return ""
	}
	cos := getDDUrl(ddurl, cookies)
	return cos

}

func ParseBuildDatadomeURL(body string, datadomeCid, datadomeURL, url string) (string, string, string, string) {
	if !strings.Contains(body, "var dd") && !strings.Contains(body, "url\":\"") {
		return "", "", "", ""
	}
	t := ""
	b := ""
	cid := ""
	hsh := ""
	if strings.Contains(body, "url\":\"") {
		m := utils.StringToJSON(body)
		if geoURL, ok := m["url"].(string); ok {
			geoURL = strings.ReplaceAll(geoURL, datadomeURL+"captcha/?", "")
			cid := strings.Split(geoURL, "&hash")[0]
			cid = strings.ReplaceAll(cid, "initialCid=", "")
			hsh = strings.Split(geoURL, "hash=")[1]
			hsh = strings.Split(hsh, "&t")[0]
			t = strings.Split(geoURL, "t=")[1]
			t = strings.Split(t, "&s")[0]
			b = strings.Split(geoURL, "s=")[1]
			b = strings.Split(b, "\"}")[0]
		}
	} else {
		re := regexp.MustCompile("var dd=([^\"]+)</script>")
		s := re.FindString(body)
		s = strings.ReplaceAll(s, "'", "\"")
		s = strings.ReplaceAll(s, "var dd=", "")
		s = strings.ReplaceAll(s, "</script>", "")
		dd := utils.StringToJSON(s)
		cid = dd["cid"].(string)
		hsh = dd["hsh"].(string)
		t = dd["t"].(string)
		b = fmt.Sprintf("%g", dd["s"].(float64))
	}
	if t == "bv" {
		return "", "", "", ""
	}
	return fmt.Sprintf("%s/captcha/?initialCid=%s&hash=%s&cid=%s&t=%s&referrer=%s&s=%s", datadomeURL, cid, hsh, datadomeCid, t, url, b), cid, hsh, b
}

func getBuildUrl(ctx context.Context, sbody, captchaUrl, datadomeURL, datadomeCid, cid, hsh, b, userAgent, pUrl, siteUrl, siteKey string, cookies []*http.Cookie) (string, []*http.Cookie) {

	req, err := http.NewRequest("GET", captchaUrl, nil)
	req.Header.Set("host", "geo.captcha-delivery.com")
	for _, co := range cookies {
		req.AddCookie(co)
	}
	if err != nil {
		return "", cookies
	}
	resp, err := connect.DefaultDo(req)
	if err != nil {
		return "", cookies
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	sbody = string(body)
	if strings.Contains(sbody, "g-recaptcha-response") {
		token := CaptchaB.GetToken()
		if token == "" {
			token = CaptchaB.GetTokenWithAPI(ctx)
			if token == "" {
				return "", cookies
			}
		}
		return strings.ReplaceAll(fmt.Sprintf("%s/captcha/check?cid=%s&icid=%s&ccid=null&g-recaptcha-response=%s&hash=%s&ua=%s&referer=%s&parent_url=%s&x-forwarded-for=%s&s=%s", datadomeURL, datadomeCid, cid, token, hsh, userAgent, siteUrl, pUrl, "", b), " ", "%20"), resp.Cookies()
	}
	return "", cookies
}

func getDDUrl(url string, cookies []*http.Cookie) string {
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("host", "geo.captcha-delivery.com")
	for _, co := range cookies {
		req.AddCookie(co)
	}
	resp, err := connect.DefaultDo(req)
	if err != nil {
		log.Println(err.Error())
		return ""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return gjson.Get(string(body), "cookie").String()
}

//}
