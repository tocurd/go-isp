package isp

import (
	"go.bug.st/serial"
	"time"
	"errors"
	"fmt"
	"bytes"
	"os"
	"io"
	"bufio"
	"strings"
	"math"
	"encoding/binary"
)

// 支持的ISP版本
var NACKError = errors.New("NACK")
var SupportedVersions = []uint8{0x31}

const WriteBlockSize = 256
const WriteMaxRetryCount = 5

type ISP struct {
	Port      serial.Port
	Supported []Command
}

type Command byte

const (
	CommandGet              Command = 0x00 // 获取当前自举程序版本及允许使用的命令
	CommandGetVersion       Command = 0x01 // 获取自举程序版本及 Flash 的读保护状态
	CommandGetID            Command = 0x02 // 获取芯片 ID
	CommandReadMemory       Command = 0x11 // 从应用程序指定的地址开始读取最多 256 个字节的存储器 空间
	CommandGo               Command = 0x21 // 跳转到内部 Flash 或 SRAM 内的应用程序代码
	CommandWriteMemory      Command = 0x31 // 从应用程序指定的地址开始将最多 256 个字节的数据写入 RAM 或 Flash
	CommandErase            Command = 0x43 // 擦除一个到全部 Flash 页面
	CommandExtendedErase    Command = 0x44 // 使用双字节寻址模式擦除一个到全部 Flash 页面（仅用于 v3.0 usart 自举程序版本及以上版本）。
	CommandWriteProtect     Command = 0x63 // 0x63 使能某些扇区的写保护
	CommandWriteUnProtect   Command = 0x73 // 0x73 禁止所有 Flash 扇区的写保护
	CommandReadoutProtect   Command = 0x82 // 0x82 使能读保护
	CommandReadoutUnprotect Command = 0x92 // 0x92 禁止读保护
)

/*
 * @Description: 激活ISP
 * @return error
 */
func (t *ISP) Activation() error {
	if err := t.Port.SetDTR(false); err != nil {
		return err
	}
	if err := t.Port.SetRTS(false); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)

	if err := t.Port.SetDTR(false); err != nil {
		return err
	}
	if err := t.Port.SetRTS(true); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if err := t.Port.SetDTR(true); err != nil {
		return err
	}
	if err := t.Port.SetRTS(false); err != nil {
		return err
	}
	if err := t.Port.SetRTS(true); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 重置状态
 * @receiver t
 * @return error
 */
func (t *ISP) Reset() error {
	if err := t.Port.SetDTR(false); err != nil {
		return err
	}
	if err := t.Port.SetRTS(true); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	if err := t.Port.SetRTS(false); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 对码
 * @return error
 */
func (t *ISP) RightCode() error {
	return t.ack([]byte{0x7F}, true, 5*time.Second)
}

/*
 * @Description: 获取bootloader版本号
 * @return error
 */
func (t *ISP) GetCommand() error {
	pack, err := t.command(CommandGet)
	if err != nil {
		return err
	}
	command := []Command{
		CommandGet, CommandGetVersion, CommandGetID, CommandReadMemory, CommandGo,
		CommandWriteMemory, CommandErase, CommandExtendedErase, CommandWriteProtect,
		CommandWriteUnProtect, CommandReadoutProtect, CommandReadoutUnprotect,
	}
	t.Supported = []Command{}
	for index := 0; index < len(command); index++ {
		if bytes.Contains(pack[2:len(pack)-1], []byte{byte(command[index])}) {
			t.Supported = append(t.Supported, command[index])
		}
	}
	return nil
}

/*
 * @Description: 获取自举程序版本号
 * @return version
 * @return option1 禁止读保护次数
 * @return option2 接收读保护使能次数
 * @return err
 */
func (t *ISP) GetVersion() (version float64, option1 uint8, option2 uint8, err error) {
	pack, err := t.command(CommandGetVersion)
	if err != nil {
		return -1, 0xFF, 0xFF, err
	}
	return float64(t.bcd2Int(pack[1])) / 10.0, pack[2], pack[3], err
}

/*
 * @Description:获取用于获取芯片 ID（标识）的版本。自举程序接收到此命令后，会将产品 ID 发送给主机。
 * @return pid
 * @return err
 */
func (t *ISP) GetID() (pid uint16, err error) {
	pack, err := t.command(CommandGetID)
	if err != nil {
		return 0xFFFF, err
	}
	pid = uint16(pack[2]) << 8
	pid += uint16(pack[3])
	return pid, err
}

/*
 * @Description: 使能写保护
 * @return error
 */
func (t *ISP) WriteProtect() error {
	if err := t.ack(t.commandBytes(CommandWriteProtect), true, 5*time.Second); err != nil {
		return err
	}
	if err := t.waitACK(100 * time.Second); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 解除写保护
 * @return error
 */
func (t *ISP) WriteUnProtect() error {
	if err := t.ack(t.commandBytes(CommandWriteUnProtect), true, 5*time.Second); err != nil {
		return err
	}
	if err := t.waitACK(100 * time.Second); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 使能读保护
 * @return error
 */
func (t *ISP) ReadoutProtect() error {
	if err := t.ack(t.commandBytes(CommandReadoutProtect), true, 5*time.Second); err != nil {
		return err
	}
	if err := t.waitACK(100 * time.Second); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 解除读保护
 * @return error
 */
func (t *ISP) ReadoutUnprotect() error {
	if err := t.ack(t.commandBytes(CommandReadoutUnprotect), true, 60*time.Second); err != nil {
		return err
	}
	if err := t.waitACK(100 * time.Second); err != nil {
		return err
	}
	return nil
}

/*
 * @Description:使用双字节寻址模式擦除一个到全部 Flash 页面（仅用于v3.0 usart 自举程序版本及以上版本）。
 * @receiver t
 */
func (t *ISP) EraseMemoryAll() error {
	if err := t.ack(t.commandBytes(CommandErase), false, 5*time.Second)
		err != nil {
		return err
	}
	if err := t.ack(t.commandBytes(0xFF), false, time.Minute); err != nil {
		return err
	}
	return nil
}

/*
 * @Description:使用双字节寻址模式擦除一个到全部 Flash 页面（仅用于v3.0 usart 自举程序版本及以上版本）。
 * @receiver t
 */
func (t *ISP) ExtendedEraseMemoryAll() error {
	if err := t.ack(t.commandBytes(CommandExtendedErase), false, 5*time.Second)
		err != nil {
		return err
	}
	if _, err := t.Port.Write([]byte{0xFF, 0xFF}); err != nil {
		return err
	}
	if err := t.ack([]byte{0x00}, false, time.Minute); err != nil {
		return err
	}
	return nil
}

/*
 * @Description: 从 RAM、Flash 和信息块中读取数据
 * @param addr 数据地址
 * @param size 读取尺寸
 * @return data 数据
 * @return err
 */
func (t *ISP) ReadMemory(addr uint64, size int) (data []byte, err error) {

	dataSize := size - 1
	if err = t.ack(t.commandBytes(CommandReadMemory), false, 5*time.Second); err != nil {
		return nil, err
	}

	temp := []byte{byte((addr >> 24) & 0xff), byte((addr >> 16) & 0xff), byte((addr >> 8) & 0xff), byte((addr) & 0xff)}
	temp = append(temp, t.checkSum(temp))
	if err = t.ack(temp, false, 5*time.Second); err != nil {
		return nil, err
	}

	if err = binary.Write(t.Port, binary.LittleEndian, []byte{byte(dataSize), byte(0xFF ^ dataSize)}); err != nil {
		return nil, err
	}

	pack, err := t.receivePack(CommandReadMemory, dataSize)
	if err != nil {
		return nil, err
	}
	return pack, nil
}

/*
 * @Description:将数据写入 RAM、Flash 或选项字节区域的任意有效存储器地址
 * @param addr 写入地址
 * @param data 写入数据
 * @return error
 */
func (t *ISP) WriteMemory(addr uint64, data []byte, verify bool) error {
	if err := t.ack(t.commandBytes(CommandWriteMemory), false, 5*time.Second); err != nil {
		return err
	}

	temp := []byte{byte((addr >> 24) & 0xff), byte((addr >> 16) & 0xff), byte((addr >> 8) & 0xff), byte((addr) & 0xff)}
	temp = append(temp, t.checkSum(temp))
	if err := t.ack(temp, false, 5*time.Second); err != nil {
		return err
	}

	// 4字节对齐
	remainder := 4 - (len(data) % 4)
	if remainder < 4 {
		for byteIndex := 0; byteIndex < remainder; byteIndex++ {
			data = append(data, 0xFF)
		}
	}

	// 下面发送数据
	temp = []byte{byte(len(data) - 1)}
	temp = append(temp, data...)
	temp = append(temp, (byte(len(data)-1))^t.checkSum(data))

	if err := binary.Write(t.Port, binary.LittleEndian, temp); err != nil {
		return err
	}
	if err := t.waitACK(5 * time.Second); err != nil {
		return err
	}

	if verify {
		memory, err := t.ReadMemory(addr, len(data))
		if err != nil {
			return err
		}

		// 284801205D030008A1010008A3010008
		// 284801205D030008A1010008A3010008
		// fmt.Printf("Read: %02X\r\n", memory[1:])
		// fmt.Printf("Write: %02X\r\n", data)

		if bytes.Equal(data, memory[1:]) == false {
			return errors.New("verify fail data writing")
		}

	}
	return nil
}

/*
 * @Description: 将文件写入flash
 * @receiver t
 * @param path
 * @return error
 */
func (t *ISP) WriteFile(addr uint64, path string, verify bool, progress func(float64)) error {

	if WriteBlockSize%4 != 0 {
		return errors.New("单次写入字节数必须为4的倍数")
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	retryCount := 0
	maxBlockCount := math.Ceil(float64(info.Size()) / float64(WriteBlockSize))
	currentBlockCount := 0

	if strings.Contains(path, ".hex") {
		scanner := bufio.NewScanner(f)
		maxBlockCount = 0
		for scanner.Scan() {
			maxBlockCount++
		}
		if err = scanner.Err(); err != nil {
			return err
		}
		_, err = f.Seek(0, 0)
		if err != nil {
			return err
		}

		data := make([]byte, WriteBlockSize)
		reader := bufio.NewReader(f)
		for {
			lineStr, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			if len(lineStr) <= 0 || lineStr[0] != ':' {
				continue
			}

			line, err := t.hexCharToBytes(strings.ReplaceAll(lineStr[1:], "\r\n", ""))
			if err != nil {
				return err
			}

			if len(line) < 4 {
				continue
			}

			progress(float64(currentBlockCount) / maxBlockCount * 100.0)
			currentBlockCount++

			dataOffset := uint64(line[1])<<8 | uint64(line[2])
			dataType := line[3]
			if dataType != 0x00 {
				continue
			}

			data = append(data, line[4:len(line)-1]...)

			// fmt.Printf("-----------\r\n")
			// fmt.Printf("line:%02X\r\n", line)
			// fmt.Printf("dataType:%02X\r\n", dataType)
			// fmt.Printf("dataOffset:%02X\r\n", dataOffset)
			// fmt.Printf("dataLen:%02X\r\n", dataLen)
			// fmt.Printf("data:%02X\r\n", line[4:len(line)-1])
			// :10 0000 00 284801205D030008A1010008A3010008A1
			// :10 0000 00 284801205D030008A1010008A3010008A1

		GoRetry1:
			if err = t.WriteMemory(addr+dataOffset, line[4:len(line)-1], verify); err != nil {
				retryCount++
				if retryCount >= WriteMaxRetryCount {
					return fmt.Errorf("write addr 0x%02X fail err:%s", 0x08000000+dataOffset, err.Error())
				}
				goto GoRetry1
			}

		}
	}

	if strings.Contains(path, ".bin") {
		// 创建一个读取器，每次读取256个字节
		offset := 0
		buffer := make([]byte, WriteBlockSize)
		reader := bufio.NewReaderSize(f, WriteBlockSize)
		for {
			progress(float64(currentBlockCount) / maxBlockCount * 100.0)
			n, err := reader.Read(buffer)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 || err == io.EOF {
				break
			}

			currentBlockCount++
		GoRetry2:
			if err = t.WriteMemory(addr+uint64(offset), buffer[:n], verify); err != nil {
				retryCount++
				if retryCount >= WriteMaxRetryCount {
					return fmt.Errorf("write addr 0x%02X fail err:%s", 0x08000000+offset, err.Error())
				}
				goto GoRetry2
			}

			offset += 256
		}
	}

	return nil
}
