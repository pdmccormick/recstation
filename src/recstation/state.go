package recstation

import (
	"io"
	"net"
	"os"
	"time"
)

type Group struct {
	Name string
	Addr net.IP
}

type State struct {
	ConfigJson

	Hostname         string
	Iface            *net.Interface
	NewOutputEvery   time.Duration
	HeartbeatTimeout time.Duration
	GroupAddrs       []net.IP
	Groups           []*Group
	ListenAddr       string
	StatusRequest    chan chan *StatusMessage
	RecordRequest    chan chan bool
	StopRequest      chan chan bool
	PreviewRequest   chan PreviewMessage

	Recording      bool
	RecordingStart time.Time
}

type StatusMessage struct {
	Hostname          string   `json:"hostname"`
	Recording         bool     `json:"recording"`
	RecordingDuration float64  `json:"recording_duration"`
	Sinks             []string `json:"sinks"`
}

type PreviewMessage struct {
	Sink   string
	Writer io.Writer
	Next   bool
	Ready  chan error
}

func MakeState(cfg ConfigJson) (*State, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	iface, err := net.InterfaceByName(cfg.IfaceName)
	if err != nil {
		return nil, err
	}

	new_output_every, err := time.ParseDuration(cfg.NewOutputEveryDur)
	if err != nil {
		return nil, err
	}

	heartbeat_timeout, err := time.ParseDuration(cfg.HeartbeatTimeoutDur)
	if err != nil {
		return nil, err
	}

	state := &State{
		ConfigJson:       cfg,
		Hostname:         hostname,
		Iface:            iface,
		NewOutputEvery:   new_output_every,
		HeartbeatTimeout: heartbeat_timeout,
		StatusRequest:    make(chan chan *StatusMessage),
		RecordRequest:    make(chan chan bool),
		StopRequest:      make(chan chan bool),
		PreviewRequest:   make(chan PreviewMessage),
	}

	for multicast, name := range state.Multicast2Name {
		addr := net.ParseIP(multicast)

		state.Groups = append(state.Groups, &Group{
			Name: name,
			Addr: addr,
		})
	}

	for _, group := range state.Groups {
		state.GroupAddrs = append(state.GroupAddrs, group.Addr)
	}

	return state, nil
}
