package isp

import (
	"fmt"
	"go.bug.st/serial"
	"log"
	"testing"
)

func TestISP_Main(t *testing.T) {
	// ports, err := enumerator.GetDetailedPortsList()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// if len(ports) == 0 {
	// 	fmt.Println("No serial ports found!")
	// 	return
	// }
	// for _, Port := range ports {
	// 	fmt.Printf("Found Port: %s\n", Port.Name)
	// 	if Port.IsUSB {
	// 		fmt.Printf("   USB ID     %s:%s\n", Port.VID, Port.PID)
	// 		fmt.Printf("   USB serial %s\n", Port.SerialNumber)
	// 	}
	// }

	mode := &serial.Mode{
		BaudRate:          115200,
		DataBits:          8,
		StopBits:          serial.OneStopBit,
		Parity:            serial.EvenParity,
		InitialStatusBits: &serial.ModemOutputBits{RTS: false, DTR: false},
	}
	port, err := serial.Open("COM4", mode)
	if err != nil {
		log.Fatal(err)
	}

	isp := ISP{Port: port}

	// ===========================================
	fmt.Println("进入ISP模式")
	if err := isp.Activation(); err != nil {
		panic(err)
	}

	// ===========================================
	fmt.Println("开始波特率对码")
	if err := isp.RightCode(); err != nil {
		panic(err)
	}

	// ===========================================
	if err := isp.GetCommand(); err != nil {
		panic(err)
	}
	fmt.Printf("获取支持指令: %02X\r\n", isp.Supported)

	// // ===========================================
	// version, _, _, err := isp.GetVersion()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("获取版本号: %.1f\r\n", version)
	//
	// // ===========================================
	// id, err := isp.GetID()
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("获取芯片PID: 0x%08X\r\n", id)

	// ===========================================
	fmt.Println("准备擦除芯片")
	if err = isp.ExtendedEraseMemoryAll(); err != nil {
		if err == NACKError {
			if err = isp.ReadoutProtect(); err != nil {
				if err == NACKError {
					fmt.Println("芯片已设读保护")
					fmt.Println("开始解除读保护（擦除中）")
					if err = isp.ReadoutUnprotect(); err != nil {
						panic(err)
					}
				} else {
					panic(err)
				}
			}
		} else {
			panic(err)
		}
	}

	// ===========================================
	fmt.Println("进入ISP模式")
	if err = isp.Activation(); err != nil {
		panic(err)
	}

	// ===========================================
	fmt.Println("开始波特率对码")
	if err = isp.RightCode(); err != nil {
		panic(err)
	}

	if err = isp.WriteFile(0x08000000, "E:/JY_F407VE_IAP_V1.0.15.hex", true, func(progress float64) {
		fmt.Printf("progress:%.2f%%\r\n", progress)
	}); err != nil {
		panic(err)
	}

	if err := isp.Reset(); err != nil {
		panic(err)
	}

	fmt.Println("任务完成")

}
