package isp

type Interface interface {
	// 激活ISP
	Activation() error

	// 波特率对码
	RightCode() error

	// 获取支持指令
	GetCommand() error

	// 获取芯片版本号
	GetVersion() (version float64, option1 uint8, option2 uint8, err error)

	// 获取芯片唯一编码
	GetID() (pid uint16, err error)

	// 使能读保护
	ReadoutProtect() error

	// 解除读保护
	ReadoutUnprotect() error

	// 双字节擦除全部
	ExtendedEraseMemory() error

	// 写入到某个地址数据
	WriteMemory(addr uint64, data []byte) error

	// 将文件写入到flash内
	WriteFile(addr uint64, path string, verify bool, progress func(float64)) error
}
