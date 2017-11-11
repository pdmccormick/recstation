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

	type MySink struct {
		Sink  *Sink
		Namer func(start bool) string
	}

	sinks := make(map[string]*MySink)

	new_output_tick := time.NewTicker(state.NewOutputEvery)

	for {
		select {
		case ev := <-heartbeat.Events:
			switch ev.Event {
			case HEARTBEAT_ONLINE:
				log.Printf("Online %s => %s", ev.Src, ev.Dst)

				key := ev.Dst.String()

				sink := MakeSink()

				mysink := &MySink{
					Sink:  sink,
					Namer: MakeFilenameMaker(state, key),
				}

				sink.OpenFile <- mysink.Namer
				source.AddSink(ev.Dst, sink)

				sinks[key] = mysink

			case HEARTBEAT_OFFLINE:
				log.Printf("OFFLINE %s => %s", ev.Src, ev.Dst)
				source.leaveGroup <- ev.Dst

				delete(sinks, ev.Dst.String())
			}

		case <-new_output_tick.C:
			for _, mysink := range sinks {
				mysink.Sink.OpenFile <- mysink.Namer
			}
		}
	}
}
