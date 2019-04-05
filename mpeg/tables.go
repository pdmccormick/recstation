package mpeg

import (
	"hash/crc32"
)

const (
	TABLE_HEADER_LENGTH         = 3
	TABLE_LONG_SUBHEADER_LENGTH = 5
	TABLE_CRC_LENGTH            = 4
)

type PrivateTable struct {
	PointerField                uint
	TableId                     uint
	Flag_SectionSyntaxIndicator bool
	Flag_PrivateIndicator       bool
	SectionLength               uint

	HasCRC32    bool
	CRC32       uint32
	ActualCRC32 uint32

	BodyLength uint
	BodyOffset uint
}

type PrivateLongTable struct {
	PrivateTable

	TableIdExtension     uint
	VersionNumber        uint
	CurrentNextIndicator bool
	SectionNumber        uint
	LastSectionNumber    uint
}

func (tbl *PrivateTable) ParseTable(tsbuf TsBuffer) bool {
	buf := tsbuf.GetPayload()
	offs := uint(0)
	remain := uint(len(buf))

	if tsbuf.GetPusi() {
		if remain < 1 {
			return false
		}

		p0 := uint(buf[offs+0])

		tbl.PointerField = p0
		offs += 1
		remain -= 1

		if remain < tbl.PointerField {
			return false
		}

		offs += tbl.PointerField
		remain -= tbl.PointerField
	}

	if remain < TABLE_HEADER_LENGTH {
		return false
	}

	startOffs := offs
	b0 := uint(buf[offs+0])
	b1 := uint(buf[offs+1])
	b2 := uint(buf[offs+2])
	offs += TABLE_HEADER_LENGTH
	remain -= TABLE_HEADER_LENGTH

	tbl.TableId = b0

	if (b1 & 0x80) != 0 {
		tbl.Flag_SectionSyntaxIndicator = true
	}

	if (b1 & 0x40) != 0 {
		tbl.Flag_PrivateIndicator = true
		tbl.HasCRC32 = true
	}

	tbl.SectionLength = ((b1 & 0x0f) << 4) | b2

	if tbl.SectionLength > remain {
		return false
	}

	tbl.BodyLength = tbl.SectionLength
	tbl.BodyOffset = offs

	if tbl.HasCRC32 {
		if tbl.SectionLength < TABLE_CRC_LENGTH {
			return false
		}

		tbl.BodyLength -= TABLE_CRC_LENGTH
	}

	offs += tbl.BodyLength
	remain -= tbl.BodyLength

	if tbl.HasCRC32 {
		tbl.ActualCRC32 = crc32.Checksum(buf[startOffs:offs], crc32.IEEETable)

		if remain < TABLE_CRC_LENGTH {
			return false
		}

		//table := crc32.MakeTable(0x04C11DB7)
		//table := crc32.MakeTable(0xEDB88320)

		c0 := uint32(buf[offs+0])
		c1 := uint32(buf[offs+1])
		c2 := uint32(buf[offs+2])
		c3 := uint32(buf[offs+3])

		tbl.CRC32 = (c0 << 24) | (c1 << 16) | (c2 << 8) | c3
	}

	return true
}

func (tbl *PrivateLongTable) ParseLongTable(tsbuf TsBuffer) bool {
	if !tbl.ParseTable(tsbuf) {
		return false
	}

	buf := tsbuf.GetPayload()[tbl.BodyOffset : tbl.BodyOffset+tbl.BodyLength]
	offs := uint(0)
	remain := tbl.BodyLength

	if remain < TABLE_LONG_SUBHEADER_LENGTH {
		return false
	}

	h0 := uint(buf[offs+0])
	h1 := uint(buf[offs+1])
	h2 := uint(buf[offs+2])
	h3 := uint(buf[offs+3])
	h4 := uint(buf[offs+4])
	offs += TABLE_LONG_SUBHEADER_LENGTH
	remain -= TABLE_LONG_SUBHEADER_LENGTH

	tbl.TableIdExtension = (h0 << 8) | h1

	tbl.VersionNumber = (h2 & 0x3e) >> 1
	if (h2 & 0x01) != 0 {
		tbl.CurrentNextIndicator = true
	}

	tbl.SectionNumber = h3
	tbl.LastSectionNumber = h4

	tbl.BodyOffset += offs
	tbl.BodyLength = remain

	return true
}
