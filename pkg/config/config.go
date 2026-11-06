/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : flag.go

* Purpose :

* Creation Date : 05-09-2017

* Last Modified : Sun 14 May 2017 05:11:39 AM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package config

var (
	FlagAllowIP = make(FlagSliceString, 0)
	FlagDenyIP  = make(FlagSliceString, 0)
)

type FlagSliceString []string

func (i *FlagSliceString) String() string {
	return ""
}

func (i *FlagSliceString) Set(value string) error {
	*i = append(*i, value)
	return nil
}
