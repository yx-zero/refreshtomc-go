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

// waitgroup
var wg sync.WaitGroup

// concurrent limiter
var limiter chan struct{}

// vars
var refreshtokens []string
var accesstokens []string
var xboxtokens []string
var xtxstokens = make(map[string]string)
var mctokens []string

// proxies list
var proxies []string

// config struct
type Config struct {
	InputFile       string `json:"input_file"`
	ProxyFile       string `json:"proxy_file"`
	OutputFile      string `json:"output_file"`
	ConcurrentLimit int    `json:"concurrent_limit"`
}

// get microsoft access token
func getaccesstoken(ref string, proxystr string, index int) {
	//use proxy
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	//make form to submit
	form := url.Values{}
	form.Add("client_id", "000000004c12ae6f")
	form.Add("refresh_token", ref)
	form.Add("grant_type", "refresh_token")
	form.Add("redirect_uri", "https://login.live.com/oauth20_desktop.srf")
	form.Add("scope", "service::user.auth.xboxlive.com::MBI_SSL")

	//make request and set header
	req, _ := http.NewRequest("POST", "https://login.live.com/oauth20_token.srf", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	//do request with proxy
	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Token #%d: error doing request: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}
	defer resp.Body.Close()

	//read response info and unmarshal
	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}
	err = json.Unmarshal([]byte(body), &bodyjson)
	if err != nil {
		fmt.Printf("❌ Token #%d: error unmarshaling response: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//read microsoft access token and append to list
	token, ok := bodyjson["access_token"].(string)
	if !ok {
		fmt.Printf("❌ Token #%d: get access token error: %v\n", index, bodyjson["error_description"])
		<-limiter
		wg.Done()
		return
	}
	fmt.Printf("✅ Token #%d: get access token success\n", index)
	accesstokens = append(accesstokens, token)
	<-limiter
	wg.Done()
}

// get xbox live token
func getxboxlive(accesstoken string, proxystr string, index int) {
	//use proxy
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	//make form to submit
	form := map[string]interface{}{
		"Properties": map[string]interface{}{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  accesstoken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}

	//marshal form
	jsonform, err := json.Marshal(form)
	if err != nil {
		fmt.Printf("❌ Token #%d:  error marshaling data: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//make request
	req, _ := http.NewRequest("POST", "https://user.auth.xboxlive.com/user/authenticate", bytes.NewBuffer(jsonform))

	//set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-xbl-contract-version", "0")

	//do request with proxy
	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Token #%d: error doing request: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}
	defer resp.Body.Close()

	//read response info and unmarshal
	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}
	err = json.Unmarshal(body, &bodyjson)
	if err != nil {
		fmt.Printf("❌ Token #%d: error unmarshaling: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//read xbox token and append to list
	fmt.Printf("✅ Token #%d:  get xbox live token success\n", index)
	xboxtokens = append(xboxtokens, bodyjson["Token"].(string))
	<-limiter
	wg.Done()
}

// get xtxs token and uhs
func getxtxstoken(xboxtoken string, proxystr string, index int) {
	//use proxy
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	//make form to submit
	form := map[string]interface{}{
		"Properties": map[string]interface{}{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xboxtoken},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}

	//marshal form
	jsonform, err := json.Marshal(form)
	if err != nil {
		fmt.Printf("❌ Token #%d: error marshaling: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//make request and set header
	req, _ := http.NewRequest("POST", "https://xsts.auth.xboxlive.com/xsts/authorize", bytes.NewBuffer(jsonform))
	req.Header.Set("Content-Type", "application/json")

	//do request with proxy
	client := http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("❌ Token #%d: error sending request: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}
	defer resp.Body.Close()

	//read response info and unmarshal
	body, _ := io.ReadAll(resp.Body)
	var bodyjson map[string]interface{}
	err = json.Unmarshal(body, &bodyjson)
	if err != nil {
		fmt.Printf("❌ Token #%d: error unmarshaling: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//extract xtxs token and uhs from response and append to list
	displayclaims := bodyjson["DisplayClaims"].(map[string]interface{})
	xui := displayclaims["xui"].([]interface{})
	firstxui := xui[0].(map[string]interface{})
	xtxstokens[bodyjson["Token"].(string)] = firstxui["uhs"].(string)
	fmt.Printf("✅ Token #%d:  get xtxs token success\n", index)
	<-limiter
	wg.Done()
}

// last step, get mctoken
// integrated more retry logic cuz this step fails a lot
func getmctoken(xtxstoken string, uhs string, index int, proxystr string) {
	//use proxy
	proxyurl, _ := url.Parse(proxystr)
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyurl),
	}

	//make identity token by combining the uhs and xtxs token
	identityToken := "XBL3.0 x=" + uhs + ";" + xtxstoken

	//make form to submit
	bodyData := map[string]interface{}{
		"identityToken": identityToken,
	}

	//marshal form
	jsonBody, err := json.Marshal(bodyData)
	if err != nil {
		fmt.Printf("❌ Token #%d: error marshaling: %v\n", index, err)
		<-limiter
		wg.Done()
		return
	}

	//retry stuff
	for {
		//make request and set header
		req, err := http.NewRequest("POST", "https://api.minecraftservices.com/authentication/login_with_xbox", bytes.NewBuffer(jsonBody))
		if err != nil {
			fmt.Printf("❌ Token #%d: error creating request: %v\n", index, err)
			<-limiter
			wg.Done()
			return
		}
		req.Header.Set("Content-Type", "application/json")

		//do request with proxy
		client := &http.Client{Transport: transport}
		resp, err := client.Do(req)
		if err != nil {
			//network error return
			fmt.Printf("❌ Token #%d: network error: %v\n", index, err)
			<-limiter
			wg.Done()
			return
		}
		defer resp.Body.Close()

		//read body info
		body, _ := io.ReadAll(resp.Body)

		//depend response status code, 429 = too many requests
		//retry if this happens, but pretty rare if use with proxies
		if resp.StatusCode == 429 {
			fmt.Printf("❌ Token #%d: rate limited (429), retrying in 5 seconds...\n", index)
			time.Sleep(5 * time.Second)
			continue
		}

		//200 = token issue, no retry
		if resp.StatusCode != 200 {
			fmt.Printf("❌ Token #%d: HTTP %d\n", index, resp.StatusCode)
			fmt.Printf("    Raw body: %s\n", body)
			<-limiter
			wg.Done()
			return
		}

		//unmarshal response
		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		if err != nil {
			fmt.Printf("❌ Token #%d: error parsing JSON: %v\n", index, err)
			fmt.Printf("    Raw body: %s\n", body)
			<-limiter
			wg.Done()
			return
		}

		//check if response data contains mctoken
		//if yes, append and complete
		if accessToken, ok := response["access_token"].(string); ok {
			mctokens = append(mctokens, accessToken)
			fmt.Printf("✅ Token #%d:  get mcToken success\n", index)
		} else {
			//if no exit, no retry
			fmt.Printf("❌ Token #%d: login_with_xbox failed, response: %s\n", index, string(body))
		}
		<-limiter
		wg.Done()
		return
	}
}

// read any file and return its []byte
func readfile(filename string) []byte {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("❌ error reading %v: %v\n", filename, err)
	}

	return file
}

// read proxy file, more complicated than readfile
func readproxy(filetoread string) (fileinfo []string) {
	//open proxy file instead of ReadFile
	file, err := os.Open(filetoread)
	if err != nil {
		fmt.Printf("❌ error opening %v: %v\n", filetoread, err)
		return
	}
	defer file.Close()

	//use scanner to read every line
	scanner := bufio.NewScanner(file)
	//append every line into fileinfo
	for scanner.Scan() {
		fileinfo = append(fileinfo, scanner.Text())
	}

	//return fileinfo
	return fileinfo
}

// write any file with the content in []string, one element one line
func writefile(filename string, content []string) {
	//use os.Create cuz easier
	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("❌ error opening %v: %v\n", filename, err)
		return
	}

	//write every element as a line into the file with Fprintln
	for _, val := range content {
		fmt.Fprintln(file, val)
	}
}

// random proxy
func randomproxy() string {
	//random a number inside the length of proxy list
	random := rand.Intn(len(proxies))
	//return the proxy list [random number]
	return proxies[random]
}

func main() {
	//start :)
	fmt.Printf("refresh to mctoken v1.0.1 starting...\n")
	fmt.Printf("made by yxzero :)\n")

	//read config
	var config Config
	fmt.Printf("reading config file...\n")
	configfile := readfile("config.json")

	//unmarshal config
	err := json.Unmarshal(configfile, &config)
	if err != nil {
		fmt.Printf("❌ error unmarshaling config file: %v\n", err)
		return
	}

	//set concurrent limit
	limiter = make(chan struct{}, config.ConcurrentLimit)

	//print config status
	fmt.Printf("\n------<>------\n")
	fmt.Printf("input file: %v\n", config.InputFile)
	fmt.Printf("proxy file: %v\n", config.ProxyFile)
	fmt.Printf("output file: %v\n", config.OutputFile)
	fmt.Printf("concurrent limit: %v\n", config.ConcurrentLimit)
	fmt.Printf("------<>------\n\n")

	//start main program
	fmt.Printf("----main program starting----\n")

	//read input file
	file := readfile(config.InputFile)

	//split input file using \n(new line symbol)
	lines := strings.Split(string(file), "\n")
	//append every line into refreshtokens list
	for _, line := range lines {
		//remove all invald characters
		line = strings.TrimSpace(line)
		//make sure the line isn't blank
		if line != "" {
			//append
			refreshtokens = append(refreshtokens, line)
		}
	}

	//read proxy file
	proxy := readproxy(config.ProxyFile)
	//add a "http://" to every proxy and append
	for _, val := range proxy {
		content := "http://" + val
		proxies = append(proxies, content)
	}

	//print stats
	fmt.Printf("got %v lines to process\n", len(refreshtokens))

	//get access token goroutine
	index := 1
	for _, val := range refreshtokens {
		wg.Add(1)
		limiter <- struct{}{}
		go getaccesstoken(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("\n----get access token complete----\n\n")

	//get xbox token goroutine
	index = 1
	for _, val := range accesstokens {
		wg.Add(1)
		limiter <- struct{}{}
		go getxboxlive(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("\n----get xbox token complete----\n\n")

	//get xtxs token goroutine
	index = 1
	for _, val := range xboxtokens {
		wg.Add(1)
		limiter <- struct{}{}
		go getxtxstoken(val, randomproxy(), index)
		index++
	}

	wg.Wait()
	fmt.Printf("\n----get xtxs token complete----\n\n")

	//get mcTokens goroutine
	index = 1
	for xtxstoken, uhs := range xtxstokens {
		wg.Add(1)
		limiter <- struct{}{}
		go getmctoken(xtxstoken, uhs, index, randomproxy())
		index++
	}

	wg.Wait()
	fmt.Printf("\n----get mctoken complete----\n\n")

	//all completed
	fmt.Printf("✅ all token process completed! saving results to output.txt...\n")

	//write results(mctokens) into output file
	writefile(config.OutputFile, mctokens)

	fmt.Printf("✅ results saved\n")
	fmt.Printf("\n----program complete! you can close this program now----\n")
	//done!

	//added time.sleep so the window doesn't close instantly
	time.Sleep(10000 * time.Hour)
}
