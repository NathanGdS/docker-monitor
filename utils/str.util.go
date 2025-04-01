package utils

var green = "\033[32m"
var Reset = "\033[0m"
var red = "\033[31m"
var yellow = "\033[33m"

func StrGreen(str string) string {
	return Reset + green + str + Reset
}

func StrRed(str string) string {
	return Reset + red + str + Reset
}

func StrYellow(str string) string {
	return Reset + yellow + str + Reset
}
