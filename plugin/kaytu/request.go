package kaytu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var ErrLogin = errors.New("your session is expired, please login into your Kaytu account")

func Ec2InstanceWastageRequest(reqBody EC2InstanceWastageRequest, accessToken string) (*EC2InstanceWastageResponse, error) {
	payloadEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/ec2-instance", bytes.NewBuffer(payloadEncoded))
	if err != nil {
		return nil, fmt.Errorf("[Ec2InstanceWastageRequest]: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	if len(accessToken) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[Ec2InstanceWastageRequest]: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[Ec2InstanceWastageRequest]: %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[Ec2InstanceWastageRequest]: %v", err)
	}

	if res.StatusCode == 403 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [Ec2InstanceWastageRequest]: %s", res.StatusCode, string(body))
	}

	response := EC2InstanceWastageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[Ec2InstanceWastageRequest]: %v", err)
	}
	return &response, nil
}
