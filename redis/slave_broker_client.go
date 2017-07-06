package redis

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"bytes"
)

type SlaveBrokerClient struct {
	slaveIP  string
	port     string
	username string
	password string
}

func NewSlaveBrokerClient(ip, port, username, password string) *SlaveBrokerClient {

	return &SlaveBrokerClient{
		slaveIP:  ip,
		port:     port,
		username: username,
		password: password,
	}
}

// call PUT /provisionslave with Instance as json body
func (client *SlaveBrokerClient) CreateSlaveInstance(instance *Instance) error {

	json, err := json.Marshal(instance)
	if err != nil {
		return err
	}

	response, err := client.doAuthenticatedRequest("PUT", "/provisionslave", json)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return client.slaveBrokerError(response)
	}

	return nil
}

// call DELETE /v2/service_instances/{instance_id} on slave node
func (client *SlaveBrokerClient) DestroySlaveInstance(instance *Instance) error {

	response, err := client.doAuthenticatedRequest("DELETE", "/v2/service_instances/" + instance.ID, nil)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		return client.slaveBrokerError(response)
	}

	return nil
}

func (client *SlaveBrokerClient) slaveBrokerError(response *http.Response) error {
	body, _ := ioutil.ReadAll(response.Body)
	formattedBody := ""
	if len(body) > 0 {
		formattedBody = fmt.Sprintf(", %s", string(body))
	}
	return errors.New(fmt.Sprintf("Slave broker error: %d%s", response.StatusCode, formattedBody))
}

func (client *SlaveBrokerClient) doAuthenticatedRequest(method, path string, body []byte) (*http.Response, error) {
	url := fmt.Sprintf("http://%s:%s%s", client.slaveIP, client.port, path)
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(client.username, client.password)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	return httpClient.Do(request)
}
