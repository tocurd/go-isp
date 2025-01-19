# go-isp
### STM32 自举程序中使用的 USART 协议


该项目为STM32微控制器自举程序的上位机ISP协议代码  
本项目已经在ubuntu、windows进行过STM32F40xx系列擦除写入测试，波特率115200能够正常工作

### 更新日志
2025-01-19 初始化项目，完成一些基础功能

### 函数功能

#### Activation 激活ISP
Activation() error  
使用前需要执行激活函数，使stm32进入自举程序

#### RightCode 波特率对码
RightCode() error  
连接成功后需要进行波特率对码同步

#### GetCommand 获取支持指令
GetCommand() error

#### GetVersion 获取芯片版本号
GetVersion() (version float64, option1 uint8, option2 uint8, err error)

#### GetID 获取芯片唯一编码
GetID() (pid uint16, err error)

#### ReadoutProtect 使能读保护
ReadoutProtect() error

#### ReadoutUnprotect 解除读保护
ReadoutUnprotect() error

#### ExtendedEraseMemory 双字节擦除全部
ExtendedEraseMemory() error

#### WriteMemory 写入到某个地址数据
WriteMemory(addr uint64, data []byte) error

#### WriteFile 将文件写入到flash内
WriteFile(addr uint64, path string, verify bool, progress func(float64)) error