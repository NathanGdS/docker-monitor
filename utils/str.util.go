package utils

var green = "\033[32m"
var reset = "\033[0m"
var red = "\033[31m"
var yellow = "\033[33m"

func StrGreen(str string) string {
	return green + str + reset
}

func StrRed(str string) string {
	return red + str + reset
}

func StrYellow(str string) string {
	return yellow + str + reset
}
