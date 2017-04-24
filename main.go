package main

import (
  "os"
)

func main() {
  switch os.Args[1] {
  case "run":
    parent()
  case "child":
    child()
  default:
    panic("no command passed")
  }
}

func must(err error) {
  if err != nil {
    panic(err)
  }
}
