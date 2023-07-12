package kubelet

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"

	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"k8s.io/klog/v2"
)

type ErrNotFound struct {
	endpoint string
}

func (err *ErrNotFound) Error() string {
	return fmt.Sprintf("%q not found", err.endpoint)
}

type KubeletClient struct {
	port            int
	deprecatedNoTLS bool
	client          *http.Client
	token           string
	host            string
	ip              string
}

func NewKubeletClient(url, token string) *KubeletClient {
	//fmt.Println("Func NewKubeletClient Called")
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	port := 10250
	deprecatedNoTLS := true

	c := &http.Client{
		Transport: transport,
	}
	return &KubeletClient{
		host:            net.JoinHostPort(url, strconv.Itoa(10250)),
		ip:              url,
		port:            port,
		client:          c,
		deprecatedNoTLS: deprecatedNoTLS,
		token:           token,
	}
}

func (kc *KubeletClient) GetSummary() (*stats.Summary, error) {
	scheme := "https"
	url := url.URL{
		Scheme: scheme,
		Host:   kc.host,
		Path:   "/stats/summary",
	}
	req, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	summary := &stats.Summary{}
	client := kc.client
	if client == nil {
		client = http.DefaultClient
	}
	err = kc.makeRequestAndGetValue(client, req.WithContext(context.TODO()), summary)
	if err != nil {
		klog.Errorln(err)
	}

	return summary, nil
}

func (kc *KubeletClient) makeRequestAndGetValue(client *http.Client, req *http.Request, value interface{}) error {

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+kc.token)
	response, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body - %v", err)
	}
	if response.StatusCode == http.StatusNotFound {
		return &ErrNotFound{req.URL.String()}
	} else if response.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed - %q, response: %q", response.Status, string(body))
	}
	var prettyJSON bytes.Buffer
	err = json.Indent(&prettyJSON, body, "", "\t")
	if err != nil {
		fmt.Println(err)
		panic(err.Error())
	}

	err = json.Unmarshal(body, value)
	if err != nil {
		return fmt.Errorf("failed to parse output. Response: %q. Error: %v", string(body), err)
	}

	return nil
}
