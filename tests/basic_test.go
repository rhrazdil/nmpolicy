/*
 * Copyright 2021 NMPolicy Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *	  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"github.com/nmstate/nmpolicy/nmpolicy"
	"github.com/nmstate/nmpolicy/nmpolicy/types"
	"github.com/nmstate/nmpolicy/nmpolicy/types/typestest"
)

func TestBasicPolicy(t *testing.T) {
	t.Run("Basic policy", func(t *testing.T) {
		testEmptyPolicy(t)
		testPolicyWithOnlyDesiredState(t)
		testPolicyWithCachedCaptureAndDesiredStateWithoutRef(t)
		testPolicyWithFilterCaptureAndDesiredStateCaptureEntryRef(t)
		testPolicyWithDesiredStateRefToCachedCapturedStates(t)
		testPolicyWithCaptureEntryExpressionRefToCachedCapturedStates(t)
		testGenerateUniqueTimestamps(t)
	})
}

func testEmptyPolicy(t *testing.T) {
	t.Run("is empty", func(t *testing.T) {
		s, err := nmpolicy.GenerateState(types.PolicySpec{}, nil, types.NoCache())

		assert.NoError(t, err)

		expectedEmptyState := types.GeneratedState{MetaInfo: types.MetaInfo{Version: "0"}}
		assert.NotEqual(t, time.Time{}, s.MetaInfo.TimeStamp)
		assert.Equal(t, expectedEmptyState, resetTimeStamp(s))
	})
}

func testPolicyWithOnlyDesiredState(t *testing.T) {
	// When a basic input with only the desired state is provided,
	// the policy just passes it as is to the output with no modifications.
	t.Run("with only desired state", func(t *testing.T) {
		stateData := []byte("name: test state")
		stateData, err := typestest.FormatYAML(stateData)
		assert.NoError(t, err)
		policySpec := types.PolicySpec{
			DesiredState: stateData,
		}

		s, err := nmpolicy.GenerateState(policySpec, nil, types.NoCache())
		assert.NoError(t, err)
		expectedState := types.GeneratedState{
			DesiredState: stateData,
			MetaInfo:     types.MetaInfo{Version: "0"},
		}
		assert.Equal(t, expectedState, resetTimeStamp(s))
	})
}

func testPolicyWithCachedCaptureAndDesiredStateWithoutRef(t *testing.T) {
	t.Run("with all captures cached and desired state that has no ref", func(t *testing.T) {
		stateData := []byte("name: test state")
		stateData, err := typestest.FormatYAML(stateData)
		assert.NoError(t, err)
		const capID0 = "cap0"
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				capID0: "my expression",
			},
			DesiredState: stateData,
		}

		cacheState := types.CachedState{
			Capture: map[string]types.CaptureState{capID0: {State: []byte("name: some captured state")}},
		}
		cacheState.Capture, err = formatCapturedStates(cacheState.Capture)
		assert.NoError(t, err)

		s, err := nmpolicy.GenerateState(
			policySpec,
			nil,
			cacheState)

		assert.NoError(t, err)
		expectedState := types.GeneratedState{
			Cache:        cacheState,
			DesiredState: stateData,
			MetaInfo:     types.MetaInfo{Version: "0"},
		}
		assert.Equal(t, expectedState, resetTimeStamp(s))
	})
}

var mainCurrentState = []byte(`
routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
  - destination: 1.1.1.0/24
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
  - destination: 2.2.2.0/24
    next-hop-address: 192.168.200.1
    next-hop-interface: eth2
    table-id: 254
  config:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
  - destination: 1.1.1.0/24
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
interfaces:
  - name: eth1
    type: ethernet
    state: up
    ipv4:
      address:
      - ip: 10.244.0.1
        prefix-length: 24
      - ip: 169.254.1.0
        prefix-length: 16
      dhcp: false
      enabled: true
  - name: eth2
    type: ethernet
    state: up
    ipv4:
      address:
      - ip: 1.2.3.4
        prefix-length: 24
      dhcp: false
      enabled: true
`)

var mainDesiredState = []byte(`
interfaces:
- name: br1
  description: Linux bridge with base interface as a port
  type: linux-bridge
  state: up
  ipv4: "{{ capture.base-iface.interfaces.0.ipv4 }}"
  bridge:
    options:
      stp:
        enabled: false
    port:
    - name: "{{ capture.base-iface.interfaces.0.name }}"
`)

var mainExpectedDesiredState = []byte(`
interfaces:
- name: br1
  description: Linux bridge with base interface as a port
  type: linux-bridge
  state: up
  ipv4:
    address:
    - ip: 10.244.0.1
      prefix-length: 24
    - ip: 169.254.1.0
      prefix-length: 16
    dhcp: false
    enabled: true
  bridge:
    options:
      stp:
        enabled: false
    port:
    - name: eth1
`)

func testPolicyWithFilterCaptureAndDesiredStateCaptureEntryRef(t *testing.T) {
	t.Run("with a eqfilter capture expression and desired state that has no ref", func(t *testing.T) {
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				"default-gw": `routes.running.destination=="0.0.0.0/0"`,
				"base-iface": `interfaces.name==capture.default-gw.routes.running.0.next-hop-interface`,
			},
			DesiredState: mainDesiredState,
		}
		obtained, err := nmpolicy.GenerateState(
			policySpec,
			mainCurrentState,
			types.CachedState{})
		assert.NoError(t, err)

		expected := types.GeneratedState{
			MetaInfo: types.MetaInfo{
				Version: "0",
			},
			DesiredState: mainExpectedDesiredState,
			Cache: types.CachedState{
				Capture: map[string]types.CaptureState{
					"default-gw": {
						State: []byte(`
routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`),
					},
					"base-iface": {
						State: []byte(`
interfaces:
- name: eth1
  type: ethernet
  state: up
  ipv4:
    address:
    - ip: 10.244.0.1
      prefix-length: 24
    - ip: 169.254.1.0
      prefix-length: 16
    dhcp: false
    enabled: true
`),
					},
				},
			},
		}

		obtained = resetTimeStamp(obtained)

		obtained, err = formatGenerateState(obtained)
		assert.NoError(t, err)

		expected, err = formatGenerateState(expected)
		assert.NoError(t, err)

		assert.Equal(t, expected, obtained)
	})
}

func testPolicyWithDesiredStateRefToCachedCapturedStates(t *testing.T) {
	t.Run("with desired state refereing a cached captured state", func(t *testing.T) {
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				"base-iface": "override me with the cache",
			},
			DesiredState: mainDesiredState,
		}
		cachedState := types.CachedState{
			Capture: map[string]types.CaptureState{
				"base-iface": {
					State: []byte(`
interfaces:
- name: eth1
  type: ethernet
  state: up
  ipv4:
    address:
    - ip: 10.244.0.1
      prefix-length: 24
    - ip: 169.254.1.0
      prefix-length: 16
    dhcp: false
    enabled: true
`),
				},
			},
		}

		obtained, err := nmpolicy.GenerateState(policySpec, mainCurrentState, cachedState)
		assert.NoError(t, err)

		expected := types.GeneratedState{
			MetaInfo: types.MetaInfo{
				Version: "0",
			},
			DesiredState: mainExpectedDesiredState,
			Cache: types.CachedState{
				Capture: map[string]types.CaptureState{
					"base-iface": {
						State: []byte(`
interfaces:
- name: eth1
  type: ethernet
  state: up
  ipv4:
    address:
    - ip: 10.244.0.1
      prefix-length: 24
    - ip: 169.254.1.0
      prefix-length: 16
    dhcp: false
    enabled: true
`),
					},
				},
			},
		}

		obtained = resetTimeStamp(obtained)

		obtained, err = formatGenerateState(obtained)
		assert.NoError(t, err)

		expected, err = formatGenerateState(expected)
		assert.NoError(t, err)

		assert.Equal(t, expected, obtained)
	})
}

func testPolicyWithCaptureEntryExpressionRefToCachedCapturedStates(t *testing.T) {
	t.Run("with a eqfilter capture expression and desired state that has no ref", func(t *testing.T) {
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				"default-gw": "override me with the cache",
				"base-iface": `interfaces.name==capture.default-gw.routes.running.0.next-hop-interface`,
			},
			DesiredState: mainDesiredState,
		}
		cachedState := types.CachedState{
			Capture: map[string]types.CaptureState{
				"default-gw": {
					State: []byte(`
routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`),
				},
			},
		}

		obtained, err := nmpolicy.GenerateState(policySpec, mainCurrentState, cachedState)
		assert.NoError(t, err)

		expected := types.GeneratedState{
			MetaInfo: types.MetaInfo{
				Version: "0",
			},
			DesiredState: mainExpectedDesiredState,
			Cache: types.CachedState{
				Capture: map[string]types.CaptureState{
					"default-gw": {
						State: []byte(`
routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`),
					},
					"base-iface": {
						State: []byte(`
interfaces:
- name: eth1
  type: ethernet
  state: up
  ipv4:
    address:
    - ip: 10.244.0.1
      prefix-length: 24
    - ip: 169.254.1.0
      prefix-length: 16
    dhcp: false
    enabled: true
`),
					},
				},
			},
		}

		obtained = resetTimeStamp(obtained)

		obtained, err = formatGenerateState(obtained)
		assert.NoError(t, err)

		expected, err = formatGenerateState(expected)
		assert.NoError(t, err)

		assert.Equal(t, expected, obtained)
	})
}

func testGenerateUniqueTimestamps(t *testing.T) {
	t.Run("with eq filter and no desired state should set unique timestamps", func(t *testing.T) {
		stateData := []byte(`routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`)

		const capID0 = "cap0"
		const capID1 = "cap1"
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				capID0: `routes.running.destination=="0.0.0.0/0"`,
				capID1: `routes.running.destination=="1.1.1.1/0"`,
			},
			DesiredState: stateData,
		}
		cacheState := types.CachedState{
			Capture: map[string]types.CaptureState{
				capID1: {
					State: []byte("{}"),
					MetaInfo: types.MetaInfo{
						Version:   "333",
						TimeStamp: time.Now(),
					},
				},
			},
		}
		beforeGenerate := time.Now()
		obtained, err := nmpolicy.GenerateState(
			policySpec,
			stateData,
			cacheState)
		assert.NoError(t, err)
		assert.Equal(t, obtained.MetaInfo.TimeStamp, obtained.Cache.Capture[capID0].MetaInfo.TimeStamp)
		assert.Equal(t, cacheState.Capture[capID1].MetaInfo, obtained.Cache.Capture[capID1].MetaInfo)
		assert.Greater(t, obtained.MetaInfo.TimeStamp.Sub(beforeGenerate), time.Duration(0))
		assert.Greater(t, obtained.Cache.Capture[capID0].MetaInfo.TimeStamp.Sub(beforeGenerate), time.Duration(0))
	})
}

func resetTimeStamp(generatedState types.GeneratedState) types.GeneratedState {
	generatedState.MetaInfo.TimeStamp = time.Time{}
	for captureID, captureState := range generatedState.Cache.Capture {
		captureState.MetaInfo.TimeStamp = time.Time{}
		generatedState.Cache.Capture[captureID] = captureState
	}
	return generatedState
}

func formatGenerateState(generatedState types.GeneratedState) (types.GeneratedState, error) {
	var err error
	generatedState.Cache.Capture, err = formatCapturedStates(generatedState.Cache.Capture)
	if err != nil {
		return generatedState, err
	}
	formatedDesiredState, err := typestest.FormatYAML(generatedState.DesiredState)
	if err != nil {
		return generatedState, err
	}
	generatedState.DesiredState = formatedDesiredState
	return generatedState, nil
}

func formatCapturedStates(capturedStates map[string]types.CaptureState) (map[string]types.CaptureState, error) {
	for captureID, captureState := range capturedStates {
		formatedYAML, err := typestest.FormatYAML(captureState.State)
		if err != nil {
			return nil, err
		}
		captureState.State = formatedYAML
		capturedStates[captureID] = captureState
	}
	return capturedStates, nil
}
