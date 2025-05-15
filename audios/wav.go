package audios

import (
	"bytes"
	"encoding/binary"
)

// WAVHeader 定义了WAV文件头结构
type WAVHeader struct {
	ChunkID       [4]byte // 'RIFF'
	ChunkSize     uint32  // 文件大小减去8字节
	Format        [4]byte // 'WAVE'
	Subchunk1ID   [4]byte // 'fmt '
	Subchunk1Size uint32  // 16 for PCM
	AudioFormat   uint16  // 1 for PCM
	NumChannels   uint16  // 声道数
	SampleRate    uint32  // 采样率
	ByteRate      uint32  // 每秒字节数 = SampleRate * NumChannels * BitsPerSample/8
	BlockAlign    uint16  // 每个样本的字节数 = NumChannels * BitsPerSample/8
	BitsPerSample uint16  // 量化位数
	Subchunk2ID   [4]byte // 'data'
	Subchunk2Size uint32  // 数据长度
}

func (h *WAVHeader) Serialize() []byte {
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.LittleEndian, h.ChunkID)
	binary.Write(&buffer, binary.LittleEndian, h.ChunkSize)
	binary.Write(&buffer, binary.LittleEndian, h.Format)
	binary.Write(&buffer, binary.LittleEndian, h.Subchunk1ID)
	binary.Write(&buffer, binary.LittleEndian, h.Subchunk1Size)
	binary.Write(&buffer, binary.LittleEndian, h.AudioFormat)
	binary.Write(&buffer, binary.LittleEndian, h.NumChannels)
	binary.Write(&buffer, binary.LittleEndian, h.SampleRate)
	binary.Write(&buffer, binary.LittleEndian, h.ByteRate)
	binary.Write(&buffer, binary.LittleEndian, h.BlockAlign)
	binary.Write(&buffer, binary.LittleEndian, h.BitsPerSample)
	binary.Write(&buffer, binary.LittleEndian, h.Subchunk2ID)
	binary.Write(&buffer, binary.LittleEndian, h.Subchunk2Size)
	return buffer.Bytes()
}

func CreateWavHeader(sampleRate, bitDepth, numChannels int) []byte {
	// 创建WAV头部
	header := &WAVHeader{
		ChunkID:       [4]byte{'R', 'I', 'F', 'F'},
		Format:        [4]byte{'W', 'A', 'V', 'E'},
		Subchunk1ID:   [4]byte{'f', 'm', 't', ' '},
		Subchunk1Size: 16,
		AudioFormat:   1,
		NumChannels:   uint16(numChannels),
		SampleRate:    uint32(sampleRate),
		ByteRate:      uint32(sampleRate * numChannels * bitDepth / 8),
		BlockAlign:    uint16(numChannels * bitDepth / 8),
		BitsPerSample: uint16(bitDepth),
		Subchunk2ID:   [4]byte{'d', 'a', 't', 'a'},
		Subchunk2Size: uint32(0x7FFFFFFF),
	}
	header.ChunkSize = 36 + header.Subchunk2Size
	return header.Serialize()
}
