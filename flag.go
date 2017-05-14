/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : flag.go

* Purpose :

* Creation Date : 05-09-2017

* Last Modified : Sun 14 May 2017 05:11:39 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import ()

var (
	flagAllowIP flagSliceString
	flagDenyIP  flagSliceString
)

type flagSliceString []string

func (i *flagSliceString) String() string {
	return ""
}

func (i *flagSliceString) Set(value string) error {
	*i = append(*i, value)
	return nil
}
