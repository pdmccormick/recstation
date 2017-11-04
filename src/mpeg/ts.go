package mpeg

const (
	MAX_PIDS = 8192
	MAX_CC   = 16

	TS_PACKET_LENGTH      = 188
	TS_MAGIC_BYTE         = 'G'
	TS_MAX_PAYLOAD_LENGTH = 184

	TEI_OFFSET                      = 1
	TEI_MASK                        = 0x80
	PUSI_OFFSET                     = 1
	PUSI_MASK                       = 0x40
	TP_OFFSET                       = 1
	TP_MASK                         = 0x20
	PID_OFFSET                      = 1
	PID_MASK0                       = 0x1f
	PID_MASK1                       = 0xff
	PID_SHIFT0                      = 8
	TSC_OFFSET                      = 3
	TSC_MASK                        = 0xC0
	TSC_SHIFT                       = 6
	ADAPTATION_CONTROL_FIELD_OFFSET = 3
	ADAPTATION_CONTROL_FIELD_MASK   = 0x30
	ADAPTATION_CONTROL_FIELD_SHIFT  = 4
	CC_OFFSET                       = 3
	CC_MASK                         = 0x0f

	ADAPTATION_PAYLOAD_PRESENT_MASK = 0x1
	ADAPTATION_FIELD_PRESENT_MASK   = 0x2
	ADAPTATION_FIELD_LENGTH         = 4
	MAX_ADAPTATION_FIELD_LENGTH     = 183
)

type TsFrame [TS_PACKET_LENGTH]byte

type TsBuffer []byte

type PID uint16
type AFC uint8
type CC uint8

func (buf TsBuffer) ToFrame(frame *TsFrame) bool {
	if len(buf) < TS_PACKET_LENGTH {
		return false
	}

	copy(buf[0:TS_PACKET_LENGTH], frame[0:TS_PACKET_LENGTH])

	return true
}

func (frame *TsFrame) ToBuffer() TsBuffer {
	return TsBuffer(frame[0:TS_PACKET_LENGTH])
}

func (buf TsBuffer) IsValid() bool {
	return len(buf) >= TS_PACKET_LENGTH && buf[0] == TS_MAGIC_BYTE
}

func (buf TsBuffer) GetTei() bool {
	b := buf[TEI_OFFSET]

	return (b & TEI_MASK) == TEI_MASK
}

func (buf TsBuffer) SetTei(tei bool) {
	val := byte(0)
	if tei {
		val = TEI_MASK
	}

	b := buf[TEI_OFFSET]

	b = (b &^ TEI_MASK) | val

	buf[TEI_OFFSET] = b
}

func (buf TsBuffer) GetPusi() bool {
	b := buf[PUSI_OFFSET]

	return (b & PUSI_MASK) == PUSI_MASK
}

func (buf TsBuffer) SetPusi(pusi bool) {
	val := byte(0)
	if pusi {
		val = PUSI_MASK
	}

	b := buf[PUSI_OFFSET]

	b = (b &^ PUSI_MASK) | val

	buf[PUSI_OFFSET] = b
}

func (buf TsBuffer) GetTp() bool {
	b := buf[TP_OFFSET]

	return (b & TP_MASK) == TP_MASK
}

func (buf TsBuffer) SetTp(tp bool) {
	val := byte(0)
	if tp {
		val = TP_MASK
	}

	b := buf[TP_OFFSET]

	b = (b &^ TP_MASK) | val

	buf[TP_OFFSET] = b
}

func (buf TsBuffer) GetPid() PID {
	b0 := buf[PID_OFFSET+0]
	b1 := buf[PID_OFFSET+1]

	return ((PID(b0) & PID_MASK0) << PID_SHIFT0) | PID(b1&PID_MASK1)
}

func (buf TsBuffer) SetPid(pid PID) {
	b0 := buf[PID_OFFSET+0]
	b1 := buf[PID_OFFSET+0]

	b0 = (b0 &^ PID_MASK0) | byte((pid&(PID_MASK0<<PID_SHIFT0))>>PID_SHIFT0)
	b1 = byte(pid & PID_MASK1)

	buf[PID_OFFSET+0] = b0
	buf[PID_OFFSET+1] = b1
}

func (buf TsBuffer) GetAfc() AFC {
	b := buf[ADAPTATION_CONTROL_FIELD_OFFSET]

	return AFC((b & ADAPTATION_CONTROL_FIELD_MASK) >> ADAPTATION_CONTROL_FIELD_SHIFT)
}

func (buf TsBuffer) SetAfc(afc AFC) {
	b := buf[ADAPTATION_CONTROL_FIELD_OFFSET]

	b = (b &^ ADAPTATION_CONTROL_FIELD_MASK) | byte((afc<<ADAPTATION_CONTROL_FIELD_SHIFT)&ADAPTATION_CONTROL_FIELD_MASK)

	buf[ADAPTATION_CONTROL_FIELD_OFFSET] = b
}

func (buf TsBuffer) GetCc() CC {
	b := buf[CC_OFFSET]

	return CC(b & CC_MASK)
}

func (buf TsBuffer) SetCc(cc CC) {
	b := buf[CC_OFFSET]

	b = (b &^ CC_MASK) | byte(cc)

	buf[CC_OFFSET] = b
}

func (buf TsBuffer) GetPayload() []byte {
	offs := TS_PACKET_LENGTH

	afc := buf.GetAfc()
	if (afc & ADAPTATION_PAYLOAD_PRESENT_MASK) != 0 {
		offs = 4

		if (afc & ADAPTATION_FIELD_PRESENT_MASK) != 0 {
			af_len := buf[ADAPTATION_FIELD_LENGTH]

			if af_len > MAX_ADAPTATION_FIELD_LENGTH {
				offs = TS_PACKET_LENGTH
			} else {
				offs += 1 + int(af_len)
			}
		}
	}

	return buf[offs:TS_PACKET_LENGTH]
}
