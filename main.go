package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var wg sync.WaitGroup

var refreshtokens []string
var accesstokens []string
var xboxtokens []string
var xtxstokens = make(map[string]string)
var mctokens []string

var proxies []string

func getaccesstoken(ref string, proxystr string, index int) {
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	form := url.Values{}
	form.Add("client_id", "000000004c12ae6f")
	form.Add("refresh_token", ref)
	form.Add("grant_type", "refresh_token")
	form.Add("redirect_uri", "https://login.live.com/oauth20_desktop.srf")
	form.Add("scope", "service::user.auth.xboxlive.com::MBI_SSL")

	req, _ := http.NewRequest("POST", "https://login.live.com/oauth20_token.srf", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Token #%d: error doing request: %v\n", index, err)
		wg.Done()
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}
	err = json.Unmarshal([]byte(body), &bodyjson)
	if err != nil {
		fmt.Printf("❌ Token #%d: error unmarshaling response: %v\n", index, err)
		wg.Done()
		return
	}

	token, ok := bodyjson["access_token"].(string)
	if !ok {
		fmt.Printf("❌ Token #%d: get access token error: %v\n", index, bodyjson["error_description"])
		wg.Done()
		return
	}
	fmt.Printf("✅ Token #%d: get access token success\n", index)
	accesstokens = append(accesstokens, token)
	wg.Done()
}

func getxboxlive(accesstoken string, proxystr string, index int) {
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	form := map[string]interface{}{
		"Properties": map[string]interface{}{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  accesstoken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	jsonform, err := json.Marshal(form)
	if err != nil {
		fmt.Printf("❌Token #%d:  error marshaling data: %v\n", index, err)
		wg.Done()
		return
	}

	req, _ := http.NewRequest("POST", "https://user.auth.xboxlive.com/user/authenticate", bytes.NewBuffer(jsonform))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-xbl-contract-version", "0")

	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Token #%d: error doing request: %v\n", index, err)
		wg.Done()
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}

	err = json.Unmarshal(body, &bodyjson)
	if err != nil {
		fmt.Printf("❌ Token #%d: 1 error unmarshaling: %v\n", index, err)
		wg.Done()
		return
	}

	fmt.Printf("✅Token #%d:  get xbox live token success\n", index)
	xboxtokens = append(xboxtokens, bodyjson["Token"].(string))
	wg.Done()
}

func getxtxstoken(xboxtoken string, proxystr string, index int) {
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	form := map[string]interface{}{
		"Properties": map[string]interface{}{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xboxtoken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	jsonform, err := json.Marshal(form)
	if err != nil {
		fmt.Printf("error marshaling: %v\n", err)
		wg.Done()
		return
	}

	req, _ := http.NewRequest("POST", "https://xsts.auth.xboxlive.com/xsts/authorize", bytes.NewBuffer(jsonform))
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌Token #%d:  error sending request: %v\n", index, err)
		wg.Done()
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}
	err = json.Unmarshal(body, &bodyjson)
	if err != nil {
		fmt.Printf("❌Token #%d:  2 error unmarshaling: %v\n", index, err)
		wg.Done()
		return
	}

	displayclaims := bodyjson["DisplayClaims"].(map[string]interface{})
	xui := displayclaims["xui"].([]interface{})
	firstxui := xui[0].(map[string]interface{})
	xtxstokens[bodyjson["Token"].(string)] = firstxui["uhs"].(string)
	fmt.Printf("✅Token #%d:  get xtxs token success\n", index)
	wg.Done()
}

func getmctoken(xtxstoken string, uhs string, index int, proxystr string) {
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	url := "https://api.minecraftservices.com/authentication/login_with_xbox"
	identityToken := "XBL3.0 x=" + uhs + ";" + xtxstoken

	bodyData := map[string]interface{}{
		"identityToken": identityToken,
	}

	jsonBody, err := json.Marshal(bodyData)
	if err != nil {
		fmt.Printf("❌ Token #%d: error marshaling: %v\n", index, err)
		wg.Done()
		return
	}

	for {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Printf("❌ Token #%d: error creating request: %v\n", index, err)
			wg.Done()
			return
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Transport: transport}
		resp, err := client.Do(req)
		if err != nil {
			if strings.Contains(err.Error(), "no such host") || strings.Contains(err.Error(), "getaddrinfo failed") {
				fmt.Printf("🌐 Token #%d: DNS error, retrying in 5 seconds...\n", index)
				time.Sleep(5 * time.Second)
				continue
			}
			fmt.Printf("❌ Token #%d: network error: %v\n", index, err)
			wg.Done()
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode == 429 {
			fmt.Printf("🚫 Token #%d: rate limited (429), retrying in 5 seconds...\n", index)
			time.Sleep(5 * time.Second)
			continue
		}

		if resp.StatusCode != 200 {
			fmt.Printf("❌ Token #%d: HTTP %d\n", index, resp.StatusCode)
			fmt.Printf("    Raw body: %s\n", body)
			wg.Done()
			return
		}

		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		if err != nil {
			fmt.Printf("❌ Token #%d: error parsing JSON: %v\n", index, err)
			fmt.Printf("    Raw body: %s\n", body)
			wg.Done()
			return
		}

		if accessToken, ok := response["access_token"].(string); ok {
			mctokens = append(mctokens, accessToken)
			fmt.Printf("✅ Token #%d: got Minecraft token: %s...\n", index, accessToken[:30])
		} else {
			fmt.Printf("❌ Token #%d: login_with_xbox failed, response: %s\n", index, string(body))
		}
		wg.Done()
		return
	}
}

func readfile(filename string) []byte {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("error reading %v: %v\n", filename, err)
	}

	return file
}

func readproxy(filetoread string) (fileinfo []string) {
	file, err := os.Open(filetoread)
	if err != nil {
		fmt.Printf("error opening %v: %v\n", filetoread, err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fileinfo = append(fileinfo, scanner.Text())
	}

	return fileinfo
}

func writefile(filename string, content []string) {
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("error opening %v: %v\n", filename, err)
		return
	}

	for _, val := range content {
		fmt.Fprintln(file, val)
	}
}

func randomproxy() string {
	random := rand.Intn(len(proxies))
	return proxies[random]
}

func main() {
	fmt.Printf("refresh to mctoken v1.0 starting...\n")

	file := readfile("input.txt")

	lines := strings.Split(string(file), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			refreshtokens = append(refreshtokens, line)
		}
	}

	proxy := readproxy("proxy.txt")
	for _, val := range proxy {
		content := "http://" + val
		proxies = append(proxies, content)
	}

	fmt.Printf("got %v lines to process\n", len(refreshtokens))

	index := 1
	for _, val := range refreshtokens {
		wg.Add(1)
		go getaccesstoken(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("----get access token complete----\n")

	index = 1
	for _, val := range accesstokens {
		wg.Add(1)
		go getxboxlive(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("----get xbox token complete----\n")

	index = 1
	for _, val := range xboxtokens {
		wg.Add(1)
		go getxtxstoken(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("----get xtxs token complete----\n")

	index = 1
	for xtxstoken, uhs := range xtxstokens {
		wg.Add(1)
		go getmctoken(xtxstoken, uhs, index, randomproxy())
		index++
	}

	wg.Wait()
	fmt.Printf("----get mctoken complete----\n")

	fmt.Printf("all token process completed! saving results to output.txt...\n")

	writefile("output.txt", mctokens)

	fmt.Printf("results saved\n")
	fmt.Printf("program complete! exiting...")
}
