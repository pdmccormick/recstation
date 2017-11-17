package recstation

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func Usage() {
	fmt.Fprint(os.Stderr, "Usage of ", os.Args[0], ":\n")
	flag.PrintDefaults()
	fmt.Fprint(os.Stderr, "\n")
}

func MakeFilenameMaker(state *State, stream string) func(bool) string {
	return func(start bool) string {
		ts := time.Now()
		year, month, day := ts.Date()

		var data = map[string]interface{}{
			"hostname":  state.Hostname,
			"stream":    stream,
			"year":      fmt.Sprintf("%04d", year),
			"month":     fmt.Sprintf("%02d", month),
			"day":       fmt.Sprintf("%02d", day),
			"timestamp": ts.Format(state.OutputTimestamp),
		}

		if start {
			data["start"] = 1
		} else {
			data["start"] = 0
		}

		return Tsprintf(state.OutputFilename, data)
	}
}

func RunMain() {
	flag.Usage = Usage
	configFilename := flag.String("config", "", "Config filename")
	httpListen := flag.String("addr", ":8080", "HTTP service address")

	flag.Parse()

	if configFilename == nil {
		Usage()
		os.Exit(1)
	}

	var cfg ConfigJson

	err := cfg.OpenJson(*configFilename)
	if err != nil {
		panic(err)
	}

	state, err := MakeState(cfg)
	if err != nil {
		panic(err)
	}

	if cfg.HttpListen != "" {
		httpListen = &cfg.HttpListen
	}

	if err := StartWeb(state, *httpListen); err != nil {
		panic(err)
	}

	source, err := MakeUdpSource(state.Iface, state.SourceListen)
	if err != nil {
		panic(err)
	}

	if err := SendPeriodicIgmpMembershipReports(state.Iface, state.GroupAddrs); err != nil {
		panic(err)
	}

	heartbeat, err := MakeHeartbeat(state.Iface, state.HeartbeatListen, state.HeartbeatTimeout, state.GroupAddrs)
	if err != nil {
		panic(err)
	}

	//func MakeAudioSource(device string, num_channels, bitrate int) *AudioSource {
	audio := MakeAudioSource(state.AlsaDevice, state.AlsaNumChannels, state.AlsaBitrate)
	log.Printf("Audio device %s", audio.Device)

	sinks := make(map[string]*Sink)

	audio.Sink = MakeSink("audio", MakeFilenameMaker(state, "audio"))

	new_output_tick := time.NewTicker(state.NewOutputEvery)
	new_output_tick.Stop()

	for {
		select {
		case ev := <-audio.Event:
			switch ev {
			case AUDIO_EVENT_STARTUP:
				sinks["audio"] = audio.Sink

				if state.Recording {
					audio.Sink.OpenFileRequest <- true
				}

			case AUDIO_EVENT_SHUTDOWN:
				audio.Sink.StopRequest <- true
				delete(sinks, "audio")
			}

		case resp := <-state.RecordRequest:
			if state.Recording {
				// Already recording
				resp <- false
			} else {
				// Begin recording
				state.Recording = true
				state.RecordingStart = time.Now()

				new_output_tick = time.NewTicker(state.NewOutputEvery)

				for _, sink := range sinks {
					sink.OpenFileRequest <- true
				}

				resp <- true
			}

		case resp := <-state.StopRequest:
			if state.Recording {
				// Stop recording
				state.Recording = false

				new_output_tick.Stop()

				for _, sink := range sinks {
					sink.StopRequest <- true
				}

				resp <- true
			} else {
				// Already stopped
				resp <- false
			}

		case resp := <-state.StatusRequest:
			st := StatusMessage{
				Hostname:          state.Hostname,
				Recording:         state.Recording,
				RecordingDuration: 0,
			}

			if state.Recording {
				st.RecordingDuration = time.Since(state.RecordingStart).Seconds()
			}

			for _, sink := range sinks {
				st.Sinks = append(st.Sinks, sink.Name)
			}

			resp <- &st

		case req := <-state.PreviewRequest:
			if sink, ok := sinks[req.Sink]; ok {
				p := sink.Preview

				if p != nil {
					p.JpegRequest <- PreviewJpegRequest{
						Next:   req.Next,
						Writer: req.Writer,
						Ready:  req.Ready,
					}

					continue
				}
			}

			req.Ready <- nil

		case ev := <-heartbeat.Events:
			switch ev.Event {
			case HEARTBEAT_ONLINE:
				key := ev.Dst.String()

				name, ok := state.Multicast2Name[key]
				if !ok {
					name = key
				}

				log.Printf("Online %s => %s (%s)", ev.Src, ev.Dst, name)

				sink := MakeSink(name, MakeFilenameMaker(state, name))

				sink.Preview = MakePreview(state.PreviewFramerate, state.PreviewWidth, state.PreviewHeight)

				source.AddSink(ev.Dst, sink)

				sinks[name] = sink

				if state.Recording {
					sink.OpenFileRequest <- true
				}

			case HEARTBEAT_OFFLINE:
				log.Printf("OFFLINE %s => %s", ev.Src, ev.Dst)
				source.leaveGroup <- ev.Dst

				source.RemoveSink(ev.Dst)

				key := ev.Dst.String()

				name, ok := state.Multicast2Name[key]
				if !ok {
					name = key
				}

				if sink, ok := sinks[name]; ok {
					sink.OfflineRequest <- true
				}

				delete(sinks, name)
			}

		case <-new_output_tick.C:
			if !state.Recording {
				continue
			}

			for _, sink := range sinks {
				sink.OpenFileRequest <- true
			}
		}
	}
}
