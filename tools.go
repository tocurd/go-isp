package isp

import (
	"fmt"
	"time"
	"errors"
)

func (t *ISP) checkSum(data []byte) byte {
	result := byte(0)
	for index := 0; index < len(data); index++ {
		result ^= data[index]
	}
	return result
}
func (t *ISP) commandBytes(command Command) []byte {
	return []byte{byte(command), 0xFF - byte(command)}
}
func (t *ISP) bcd2Int(bcd byte) uint8 {
	return ((bcd / 0x10) * 10) + (bcd % 0x10)
}
func (t *ISP) int2BCD(num uint8) byte {
	return ((num % 100) / 10 << 4) + num%10
}

/*
 * @Description: 接收数据
 * @receiver t
 */
func (t *ISP) receivePack(command Command) ([]byte, error) {
	timeout := time.After(5 * time.Second)
	buff := make([]byte, 0)
	for {
		select {
		case <-timeout:
			return nil, errors.New("receive timeout")
		default:
			temp := make([]byte, 100)
			n, err := t.Port.Read(temp)
			if err != nil {
				return nil, err
			}
			if n <= 0 {
				continue
			}

			buff = append(buff, temp[:n]...)
			for index := 0; index < len(buff); index++ {

				// 接收到包开始
				if buff[index] == 0x79 && index+1 < len(buff) {
					packLen := int(buff[index+1]) + index + 4

					if command == CommandGetVersion {
						packLen = 2 + index + 3
					}

					// 包长度符合要求
					if len(buff) >= packLen {
						pack := buff[index:packLen]
						if pack[0] == 0x79 && pack[len(pack)-1] == 0x79 {
							return pack, nil
						}
					}
				}

				if buff[index] == 0x1F {
					buff = buff[index:]
					return nil, NACKError
				}
			}
		}
	}
}

/*
 * @Description: 等待确认帧
 * @return error
 */
func (t *ISP) ack(data []byte, retry bool, after time.Duration) error {
	timeout := time.After(after)
	flag := 0
	for {
		select {
		case <-timeout:
			return fmt.Errorf("0x%02X timeout", data)
		default:
			if flag == 0 {
				if _, err := t.Port.Write(data); err != nil {
					return err
				}
			}
			if retry == false {
				flag = 1
			}
			if err := t.waitACK(300 * time.Millisecond); err != nil {
				if err.Error() == "timeout" {
					continue
				}
				return err
			}

			return nil
		}
	}
}

func (t *ISP) waitACK(after time.Duration) error {
	timeout := time.After(after)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout")
		default:
			buff := make([]byte, 8)
			n, err := t.Port.Read(buff)
			if err != nil {
				return err
			}
			if n > 0 {
				for index := 0; index < n; index++ {
					if buff[index] == 0x79 {
						return nil
					}
					if buff[index] == 0x1F {
						return NACKError
					}
				}
			}
		}
	}
}

/*
 * @Description: 执行指令
 * @param command
 * @return []byte
 * @return error
 */
func (t *ISP) command(command Command) ([]byte, error) {
	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("command 0x%02X timeout", command)
		default:
			data := []byte{byte(command), byte(0xFF - command)}

			if _, err := t.Port.Write(data); err != nil {
				return nil, err
			}
			pack, err := t.receivePack(command)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return pack, nil
		}
	}
}
