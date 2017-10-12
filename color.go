package pool-controller

import (
	"fmt"
)

TABLEAU_20:= [[ 31, 119, 180], [174, 199, 232], [255, 127,  14], [255, 187, 120],
              [ 44, 160,  44], [152, 223, 138], [214,  39,  40], [255, 152, 150],
              [148, 103, 189], [197, 176, 213], [140,  86,  75], [196, 156, 148],
              [227, 119, 194], [247, 182, 210], [127, 127, 127], [199, 199, 199],
              [188, 189,  34], [219, 219, 141], [ 23, 190, 207], [158, 218, 229]]

func colorStr(count) {
	return fmt.Sprintf("#%02x%02x%02x", TABLEAU_20[count%20][0],
		TABLEAU_20[count%20][1], TABLEAU_20[count%20][2])
}
