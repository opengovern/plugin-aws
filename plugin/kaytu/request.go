package kaytu

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var ErrLogin = errors.New("your session is expired, please login")

func Ec2InstanceWastageRequest(reqBody EC2InstanceWastageRequest, token string) (*EC2InstanceWastageResponse, error) {
	payloadEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/ec2-instance", bytes.NewBuffer(payloadEncoded))
	//req, err := http.NewRequest("POST", "http://localhost:8000/api/v1/wastage/ec2-instance", bytes.NewBuffer(payloadEncoded))
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	req.Header.Add("content-type", "application/json")
	if len(token) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	if res.StatusCode == 403 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [requestAbout] : %s", res.StatusCode, string(body))
	}

	response := EC2InstanceWastageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	return &response, nil
}

func RDSInstanceWastageRequest(reqBody AwsRdsWastageRequest, token string) (*AwsRdsWastageResponse, error) {
	payloadEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/aws-rds", bytes.NewBuffer(payloadEncoded))
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	req.Header.Add("content-type", "application/json")
	if len(token) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	if res.StatusCode == 403 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [requestAbout] : %s", res.StatusCode, string(body))
	}

	response := AwsRdsWastageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	return &response, nil
}

func RDSClusterWastageRequest(reqBody AwsClusterWastageRequest, token string) (*AwsClusterWastageResponse, error) {
	payloadEncoded, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/aws-rds-cluster", bytes.NewBuffer(payloadEncoded))
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	req.Header.Add("content-type", "application/json")
	if len(token) > 0 {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}

	if res.StatusCode == 403 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [requestAbout] : %s", res.StatusCode, string(body))
	}

	response := AwsClusterWastageResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[requestAbout] : %v", err)
	}
	return &response, nil
}

func ConfigurationRequest() (*Configuration, error) {
	req, err := http.NewRequest("POST", "https://api.kaytu.io/kaytu/wastage/api/v1/wastage/configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	req.Header.Add("content-type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	err = res.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}

	if res.StatusCode == 403 {
		return nil, ErrLogin
	}

	if res.StatusCode >= 300 || res.StatusCode < 200 {
		return nil, fmt.Errorf("server returned status code %d, [ConfigurationRequest]: %s", res.StatusCode, string(body))
	}

	response := Configuration{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("[ConfigurationRequest]: %v", err)
	}
	return &response, nil
}
