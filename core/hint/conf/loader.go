// Package conf provides a v2raya-core JSON configuration loader.
// It extends xray-core's JSON loader with support for multiObservatory,
// and automatically injects the v2ray-compatible observatory gRPC service.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.
package conf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	xray_commander "github.com/xtls/xray-core/app/commander"
	xray_core "github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/cmdarg"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/serial"
	xray_conf "github.com/xtls/xray-core/infra/conf"
	xray_cfgdur "github.com/xtls/xray-core/infra/conf/cfgcommon/duration"
	conf_serial "github.com/xtls/xray-core/infra/conf/serial"
	"github.com/xtls/xray-core/main/confloader"
	compat "github.com/v2rayA/v2raya-core/hint/app/observatory/compat"
	multiobs "github.com/v2rayA/v2raya-core/hint/app/observatory/multiobservatory"
)

// multiObsEntryJSON represents one observer group in the JSON config.
type multiObsEntryJSON struct {
	Tag             string                  `json:"tag"`
	ProbeURL        string                  `json:"probeURL"`
	ProbeInterval   xray_cfgdur.Duration    `json:"probeInterval"`
	SubjectSelector []string                `json:"subjectSelector"`
}

// multiObsJSON is the top-level JSON structure for the multiObservatory field.
type multiObsJSON struct {
	Observers []multiObsEntryJSON `json:"observers"`
}

// extendedJSON captures v2raya-core extension fields alongside standard xray JSON.
type extendedJSON struct {
	MultiObservatory *multiObsJSON `json:"multiObservatory"`
}

// injectMultiObservatory appends the multiobservatory.Config TypedMessage to coreConfig.App.
func injectMultiObservatory(coreConfig *xray_core.Config, mo *multiObsJSON) {
	cfg := &multiobs.Config{}
	for _, e := range mo.Observers {
		cfg.Observers = append(cfg.Observers, &multiobs.ObserverConfig{
			Tag:             e.Tag,
			ProbeUrl:        e.ProbeURL,
			ProbeInterval:   int64(e.ProbeInterval),
			SubjectSelector: e.SubjectSelector,
		})
	}
	coreConfig.App = append(coreConfig.App, serial.ToTypedMessage(cfg))
}

// injectCompatService adds compat.CompatConfig to the commander's service list.
// If no commander is found (no api section), this is a no-op.
func injectCompatService(coreConfig *xray_core.Config) {
	const commanderType = "xray.app.commander.Config"
	for i, app := range coreConfig.App {
		if app.Type != commanderType {
			continue
		}
		msg, err := app.GetInstance()
		if err != nil {
			continue
		}
		cmdCfg, ok := msg.(*xray_commander.Config)
		if !ok {
			continue
		}
		cmdCfg.Service = append(cmdCfg.Service, serial.ToTypedMessage(&compat.CompatConfig{}))
		coreConfig.App[i] = serial.ToTypedMessage(cmdCfg)
		return
	}
}

// loadAndExtend reads a config file, decodes it as xray conf.Config, and
// also extracts the extendedJSON fields (e.g. multiObservatory).
func loadAndExtend(arg string) (*xray_conf.Config, *extendedJSON, error) {
	r, err := confloader.LoadConfig(arg)
	if err != nil {
		return nil, nil, errors.New("failed to read config: ", arg).Base(err)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, errors.New("failed to read config bytes: ", arg).Base(err)
	}
	c, err := conf_serial.DecodeJSONConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, errors.New("failed to decode config: ", arg).Base(err)
	}
	ext := &extendedJSON{}
	// Ignore JSON errors here — unknown fields in extendedJSON are fine.
	_ = json.Unmarshal(raw, ext)
	return c, ext, nil
}

func init() {
	common.Must(xray_core.RegisterConfigLoader(&xray_core.ConfigFormat{
		Name:      "JSON",
		Extension: []string{"json"},
		Loader: func(input interface{}) (*xray_core.Config, error) {
			var ext *extendedJSON

			switch v := input.(type) {
			case cmdarg.Arg:
				cf := &xray_conf.Config{}
				for i, arg := range v {
					errors.LogInfo(context.Background(), "v2raya-core: reading config: ", arg)
					c, e, err := loadAndExtend(arg)
					if err != nil {
						return nil, err
					}
					if i == 0 {
						*cf = *c
					} else {
						cf.Override(c, arg)
					}
					// Use extensions from the last file that defines them.
					if e != nil && e.MultiObservatory != nil {
						ext = e
					}
				}
				coreConfig, err := cf.Build()
				if err != nil {
					return nil, err
				}
				if ext != nil && ext.MultiObservatory != nil {
					injectMultiObservatory(coreConfig, ext.MultiObservatory)
				}
				injectCompatService(coreConfig)
				return coreConfig, nil

			case io.Reader:
				raw, err := io.ReadAll(v)
				if err != nil {
					return nil, errors.New("failed to read config reader").Base(err)
				}
				c, err := conf_serial.DecodeJSONConfig(bytes.NewReader(raw))
				if err != nil {
					return nil, errors.New("failed to decode JSON config").Base(err)
				}
				e := &extendedJSON{}
				_ = json.Unmarshal(raw, e)
				coreConfig, err := c.Build()
				if err != nil {
					return nil, err
				}
				if e.MultiObservatory != nil {
					injectMultiObservatory(coreConfig, e.MultiObservatory)
				}
				injectCompatService(coreConfig)
				return coreConfig, nil

			default:
				return nil, errors.New("unknown config input type")
			}
		},
	}))
}
