package mpeg

const (
	PAT_SUBHEADER_LENGTH = 5
	PAT_ENTRY_LENGTH     = 4

	PAT_MAX_ENTRIES = ((TS_MAX_PAYLOAD_LENGTH - PAT_SUBHEADER_LENGTH) / PAT_ENTRY_LENGTH) + 1
)

type PAT struct {
	PrivateTable

	TransportStreamId    uint
	VersionNumber        uint
	CurrentNextIndicator bool
	SectionNumber        uint
	LastSectionNumber    uint

	NumEntry int
	Entry    [PAT_MAX_ENTRIES]PATEntry
}

type PATEntry struct {
	ProgramNumber uint

	Flag_Network  bool
	NetworkPID    PID
	Flag_PMT      bool
	ProgramMapPID PID
}

func (pat *PAT) ParsePAT(tsbuf TsBuffer) bool {
	if tsbuf.GetPid() != PID_PAT {
		return false
	}

	if !pat.ParseTable(tsbuf) {
		return false
	}

	buf := tsbuf.GetPayload()[pat.BodyOffset : pat.BodyOffset+pat.BodyLength]
	offs := uint(0)
	remain := pat.BodyLength

	if remain < PAT_SUBHEADER_LENGTH {
		return false
	}

	h0 := uint(buf[offs+0])
	h1 := uint(buf[offs+1])
	h2 := uint(buf[offs+2])
	h3 := uint(buf[offs+3])
	h4 := uint(buf[offs+4])
	offs += PAT_SUBHEADER_LENGTH
	remain -= PAT_SUBHEADER_LENGTH

	pat.TransportStreamId = (h0 << 8) | h1

	pat.VersionNumber = (h2 & 0x3e) >> 1
	if (h2 & 0x01) != 0 {
		pat.CurrentNextIndicator = true
	}

	pat.SectionNumber = h3
	pat.LastSectionNumber = h4

	for i := 0; remain >= PAT_ENTRY_LENGTH && i < PAT_MAX_ENTRIES; {
		e0 := uint(buf[offs+0])
		e1 := uint(buf[offs+1])
		e2 := uint(buf[offs+2])
		e3 := uint(buf[offs+3])
		offs += PAT_ENTRY_LENGTH
		remain -= PAT_ENTRY_LENGTH

		programNumber := (e0 << 8) | e1

		if programNumber == 0xFFFF {
			continue
		}

		entry := &pat.Entry[i]
		entry.ProgramNumber = programNumber

		next := PID(((e2 & 0x1f) << 8) | e3)

		if entry.ProgramNumber == 0 {
			entry.Flag_Network = true
			entry.NetworkPID = next
		} else {
			entry.Flag_PMT = true
			entry.ProgramMapPID = next
		}

		i++
		pat.NumEntry++
	}

	return true
}
