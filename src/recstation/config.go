package recstation

import (
	"encoding/json"
	"io"
	"os"
)

type ConfigJson struct {
	IfaceName           string            `json:"iface"`
	OutputFilename      string            `json:"output_filename"`
	OutputTimestamp     string            `json:"output_timestamp"`
	Multicast2Name      map[string]string `json:"multicasts"`
	NewOutputEveryDur   string            `json:"new_output_every"`
	SourceListen        string            `json:"source_listen"`
	HeartbeatListen     string            `json:"heartbeat_listen"`
	HeartbeatTimeoutDur string            `json:"heartbeat_timeout"`
	HttpListen          string            `json:"http_listen"`
	AlsaDevice          string            `json:"alsa_device"`
	AlsaNumChannels     int               `json:"alsa_num_channels"`
	AlsaBitrate         int               `json:"alsa_bitrate"`
}

func (cfg *ConfigJson) DecodeJson(r io.Reader) error {
	dec := json.NewDecoder(r)

	err := dec.Decode(cfg)

	return err
}

func (cfg *ConfigJson) OpenJson(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	defer f.Close()

	return cfg.DecodeJson(f)
}
