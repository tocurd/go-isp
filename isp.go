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
 * @Description: 使能读保护
 * @return error
 */
func (t *ISP) ReadoutProtect() error {
	if err := t.ack(t.commandBytes(CommandReadoutProtect), true, 60*time.Second); err != nil {
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
func (t *ISP) ExtendedEraseMemory() error {
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
 * @Description:将数据写入 RAM、Flash 或选项字节区域的任意有效存储器地址
 * @param addr 写入地址
 * @param data 写入数据
 * @return error
 */
func (t *ISP) WriteMemory(addr uint64, data []byte) error {
	if err := t.ack([]byte{0x31, 0xCE}, false, 5*time.Second); err != nil {
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
	return nil

	// :10 0000 00 284801205D030008A1010008A3010008 A1
	// :10 0020 00 000000000000000000000000CB010008 FC
	// :10 0150 00 77030008770300087703000877030008 97

	// 0x10 = 16个数据
	// 0x0150 = 数据地址
	// 0x00 = 数据类型
	//        '00'数据记录：用来记录数据，HEX文件的大部分记录都是数据记录；
	//        '01'文件结束记录：用来标识文件结束，放在文件的最后，标识HEX文件的结尾；
	//        '02'扩展段地址记录：用来标识扩展段地址的记录；
	//        '03'开始段地址记录：开始段地址记录；
	//        '04'扩展线性地址记录：用来标识扩展线性地址的记录；
	//        '05'开始线性地址记录：开始线性地址记录；
	// 0x77030008770300087703000877030008 = 数据
	// 0x97 校验和，公式为：FF前面所有字节的加和取反然后加1，取低八位。

}

/*
 * @Description: 将文件写入flash
 * @receiver t
 * @param path
 * @return error
 */
func (t *ISP) WriteFile(addr uint64, path string, verify bool, progress func(float64)) error {
	const WriteBlockSize = 256
	const WriteMaxRetryCount = 5

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

		GoRetry:
			if err = t.WriteMemory(addr+uint64(offset), buffer[:n]); err != nil {
				if err == NACKError {
					retryCount++
					if retryCount >= WriteMaxRetryCount {
						return fmt.Errorf("write addr 0x%02X fail", 0x08000000+offset)
					}
					goto GoRetry
				}
				return err
			}
			offset += 256
		}
	}

	return nil
}
