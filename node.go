// Copyright 2015 Vadim Kravcenko
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package gojenkins

import (
	"context"
	"errors"
)

// Nodes

type Computers struct {
	BusyExecutors  int             `json:"busyExecutors"`
	Computers      []*NodeResponse `json:"computer"`
	DisplayName    string          `json:"displayName"`
	TotalExecutors int             `json:"totalExecutors"`
}

type Node struct {
	Raw     *NodeResponse
	Jenkins *Jenkins
	Base    string
}

type NodeResponse struct {
	Class          string        `json:"_class"`
	Actions        []interface{} `json:"actions"`
	DisplayName    string        `json:"displayName"`
	Description    string        `json:"description"`
	AssignedLabels []struct {
		Name string `json:"name"`
	} `json:"assignedLabels"`
	Executors []struct {
		CurrentExecutable struct {
			Number    int    `json:"number"`
			URL       string `json:"url"`
			SubBuilds []struct {
				Abort             bool        `json:"abort"`
				Build             interface{} `json:"build"`
				BuildNumber       int         `json:"buildNumber"`
				Duration          string      `json:"duration"`
				Icon              string      `json:"icon"`
				JobName           string      `json:"jobName"`
				ParentBuildNumber int         `json:"parentBuildNumber"`
				ParentJobName     string      `json:"parentJobName"`
				PhaseName         string      `json:"phaseName"`
				Result            string      `json:"result"`
				Retry             bool        `json:"retry"`
				URL               string      `json:"url"`
			} `json:"subBuilds"`
		} `json:"currentExecutable"`
	} `json:"executors"`
	Icon                string   `json:"icon"`
	IconClassName       string   `json:"iconClassName"`
	Idle                bool     `json:"idle"`
	JnlpAgent           bool     `json:"jnlpAgent"`
	LaunchSupported     bool     `json:"launchSupported"`
	LoadStatistics      struct{} `json:"loadStatistics"`
	ManualLaunchAllowed bool     `json:"manualLaunchAllowed"`
	MonitorData         struct {
		Hudson_NodeMonitors_ArchitectureMonitor interface{} `json:"hudson.node_monitors.ArchitectureMonitor"`
		Hudson_NodeMonitors_ClockMonitor        interface{} `json:"hudson.node_monitors.ClockMonitor"`
		Hudson_NodeMonitors_DiskSpaceMonitor    interface{} `json:"hudson.node_monitors.DiskSpaceMonitor"`
		Hudson_NodeMonitors_ResponseTimeMonitor struct {
			Average int64 `json:"average"`
		} `json:"hudson.node_monitors.ResponseTimeMonitor"`
		Hudson_NodeMonitors_SwapSpaceMonitor      interface{} `json:"hudson.node_monitors.SwapSpaceMonitor"`
		Hudson_NodeMonitors_TemporarySpaceMonitor interface{} `json:"hudson.node_monitors.TemporarySpaceMonitor"`
	} `json:"monitorData"`
	NumExecutors       int64         `json:"numExecutors"`
	Offline            bool          `json:"offline"`
	OfflineCause       struct{}      `json:"offlineCause"`
	OfflineCauseReason string        `json:"offlineCauseReason"`
	OneOffExecutors    []interface{} `json:"oneOffExecutors"`
	TemporarilyOffline bool          `json:"temporarilyOffline"`
}

func (n *Node) Info(ctx context.Context) (*NodeResponse, error) {
	_, err := n.Poll(ctx)
	if err != nil {
		return nil, err
	}
	return n.Raw, nil
}

func (n *Node) GetName() string {
	return n.Raw.DisplayName
}

func (n *Node) Delete(ctx context.Context) (bool, error) {
	resp, err := n.Jenkins.Requester.Post(ctx, n.Base+"/doDelete", nil, nil, nil)
	if err != nil {
		return false, err
	}
	return resp.StatusCode == 200, nil
}

func (n *Node) IsOnline(ctx context.Context) (bool, error) {
	_, err := n.Poll(ctx)
	if err != nil {
		return false, err
	}
	return !n.Raw.Offline, nil
}

func (n *Node) IsTemporarilyOffline(ctx context.Context) (bool, error) {
	_, err := n.Poll(ctx)
	if err != nil {
		return false, err
	}
	return n.Raw.TemporarilyOffline, nil
}

func (n *Node) IsIdle(ctx context.Context) (bool, error) {
	_, err := n.Poll(ctx)
	if err != nil {
		return false, err
	}
	return n.Raw.Idle, nil
}

func (n *Node) IsJnlpAgent(ctx context.Context) (bool, error) {
	_, err := n.Poll(ctx)
	if err != nil {
		return false, err
	}
	return n.Raw.JnlpAgent, nil
}

func (n *Node) SetOnline(ctx context.Context) (bool, error) {
	_, err := n.Poll(ctx)

	if err != nil {
		return false, err
	}

	if n.Raw.Offline && !n.Raw.TemporarilyOffline {
		return false, errors.New("Node is Permanently offline, can't bring it up")
	}

	if n.Raw.Offline && n.Raw.TemporarilyOffline {
		return n.ToggleTemporarilyOffline(ctx)
	}

	return true, nil
}

func (n *Node) SetOffline(ctx context.Context, options ...interface{}) (bool, error) {
	if !n.Raw.Offline {
		return n.ToggleTemporarilyOffline(ctx, options...)
	}
	return false, errors.New("Node already Offline")
}

func (n *Node) ToggleTemporarilyOffline(ctx context.Context, options ...interface{}) (bool, error) {
	state_before, err := n.IsTemporarilyOffline(ctx)
	if err != nil {
		return false, err
	}
	qr := map[string]string{"offlineMessage": "requested from gojenkins"}
	if len(options) > 0 {
		qr["offlineMessage"] = options[0].(string)
	}
	_, err = n.Jenkins.Requester.Post(ctx, n.Base+"/toggleOffline", nil, nil, qr)
	if err != nil {
		return false, err
	}
	new_state, err := n.IsTemporarilyOffline(ctx)
	if err != nil {
		return false, err
	}
	if state_before == new_state {
		return false, errors.New("Node state not changed")
	}
	return true, nil
}

func (n *Node) ChangeOfflineCause(ctx context.Context, options ...interface{}) (bool, error) {
	if !n.Raw.TemporarilyOffline {
		return false, errors.New("node is not offline, can't change offline cause")
	}

	if len(options) < 0 {
		return false, errors.New("no offline cause provided")
	}
	newOfflineMsg := options[0].(string)
	qr := map[string]string{
		"offlineMessage": newOfflineMsg,
		"Submit":         "",
	}
	_, err := n.Jenkins.Requester.Post(ctx, n.Base+"/changeOfflineCause", nil, nil, qr)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (n *Node) Poll(ctx context.Context) (int, error) {
	response, err := n.Jenkins.Requester.GetJSON(ctx, n.Base, n.Raw, nil)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (n *Node) LaunchNodeBySSH(ctx context.Context) (int, error) {
	qr := map[string]string{
		"json":   "",
		"Submit": "Launch slave agent",
	}
	response, err := n.Jenkins.Requester.Post(ctx, n.Base+"/launchSlaveAgent", nil, nil, qr)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (n *Node) Disconnect(ctx context.Context) (int, error) {
	qr := map[string]string{
		"offlineMessage": "",
		"json":           makeJson(map[string]string{"offlineMessage": ""}),
		"Submit":         "Yes",
	}
	response, err := n.Jenkins.Requester.Post(ctx, n.Base+"/doDisconnect", nil, nil, qr)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, nil
}

func (n *Node) GetLogText(ctx context.Context) (string, error) {
	var log string

	_, err := n.Jenkins.Requester.Post(ctx, n.Base+"/log", nil, nil, nil)
	if err != nil {
		return "", err
	}

	_, err = n.Jenkins.Requester.Get(ctx, n.Base+"/logText/progressiveHtml/", &log, nil)
	if err != nil {
		return "", err
	}

	return log, nil
}

func (n *Node) GetDescription(ctx context.Context) string {
	return n.Raw.Description
}

func (n *Node) SubmitDescription(ctx context.Context, description string) error {
	_, err := n.Jenkins.Requester.Post(ctx, n.Base+"/submitDescription", nil, nil, map[string]string{"description": description})
	if err != nil {
		return err
	}
	return nil
}
