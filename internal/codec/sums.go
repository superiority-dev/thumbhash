package codec

import "fmt"

// sumColumn returns, for each row r below rows, the sum over the w-wide channel
// ch in pixel order of ch[i]*fx[x*cosStride+cx]*fy[r*h+y]; fy is exactly 1 on
// row 0.
func sumColumn(ch, fx, fy []float64, w, h, cx, rows int) (s [maxRows]float64) {
	col := cx & cosMask

	// One case per row count: each row sums into its own variable in one pass
	// over the pixels, so the rows' additions do not wait on one another.
	switch rows {
	case 1:
		var a0 float64

		for y := range h {
			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
			}
		}

		s = [maxRows]float64{a0}
	case 2:
		var a0, a1 float64

		for y := range h {
			f1 := fy[1*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
			}
		}

		s = [maxRows]float64{a0, a1}
	case 3:
		var a0, a1, a2 float64

		for y := range h {
			f1, f2 := fy[1*h+y], fy[2*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
			}
		}

		s = [maxRows]float64{a0, a1, a2}
	case 4:
		var a0, a1, a2, a3 float64

		for y := range h {
			f1, f2, f3 := fy[1*h+y], fy[2*h+y], fy[3*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
			}
		}

		s = [maxRows]float64{a0, a1, a2, a3}
	case 5:
		var a0, a1, a2, a3, a4 float64

		for y := range h {
			f1, f2, f3, f4 := fy[1*h+y], fy[2*h+y], fy[3*h+y], fy[4*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
				a4 += float64(t * f4)
			}
		}

		s = [maxRows]float64{a0, a1, a2, a3, a4}
	case 6:
		var a0, a1, a2, a3, a4, a5 float64

		for y := range h {
			f1, f2, f3, f4, f5 := fy[1*h+y], fy[2*h+y], fy[3*h+y], fy[4*h+y], fy[5*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
				a4 += float64(t * f4)
				a5 += float64(t * f5)
			}
		}

		s = [maxRows]float64{a0, a1, a2, a3, a4, a5}
	case 7:
		var a0, a1, a2, a3, a4, a5, a6 float64

		for y := range h {
			f1, f2, f3, f4, f5, f6 := fy[1*h+y], fy[2*h+y], fy[3*h+y], fy[4*h+y], fy[5*h+y], fy[6*h+y]

			for x, v := range ch[y*w : y*w+w] {
				t := float64(v * fx[x*cosStride+col])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
				a4 += float64(t * f4)
				a5 += float64(t * f5)
				a6 += float64(t * f6)
			}
		}

		s = [maxRows]float64{a0, a1, a2, a3, a4, a5, a6}
	}

	return s
}

// sumPair is sumColumn for column cx with rowsA rows and column cx+1 with rowsB
// rows in one pass, for the six pairs of row counts transform forms at every
// size up to format.MaxSize: 2 and 1, 2 and 2, 3 and 2, 3 and 3, 4 and 2, and 4
// and 3. It panics on any other pair.
func sumPair(ch, fx, fy []float64, w, h, cx, rowsA, rowsB int) (sa, sb [maxRows]float64) {
	colA, colB := cx&cosMask, (cx+1)&cosMask

	pair := rowsA*cosStride + rowsB

	switch pair {
	case 2*cosStride + 1:
		var a0, a1, b0 float64

		for y := range h {
			f1 := fy[1*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				b0 += u
			}
		}

		sa, sb = [maxRows]float64{a0, a1}, [maxRows]float64{b0}
	case 2*cosStride + 2:
		var a0, a1, b0, b1 float64

		for y := range h {
			f1 := fy[1*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				b0 += u
				b1 += float64(u * f1)
			}
		}

		sa, sb = [maxRows]float64{a0, a1}, [maxRows]float64{b0, b1}
	case 3*cosStride + 2:
		var a0, a1, a2, b0, b1 float64

		for y := range h {
			f1, f2 := fy[1*h+y], fy[2*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				b0 += u
				b1 += float64(u * f1)
			}
		}

		sa, sb = [maxRows]float64{a0, a1, a2}, [maxRows]float64{b0, b1}
	case 3*cosStride + 3:
		var a0, a1, a2, b0, b1, b2 float64

		for y := range h {
			f1, f2 := fy[1*h+y], fy[2*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				b0 += u
				b1 += float64(u * f1)
				b2 += float64(u * f2)
			}
		}

		sa, sb = [maxRows]float64{a0, a1, a2}, [maxRows]float64{b0, b1, b2}
	case 4*cosStride + 2:
		var a0, a1, a2, a3, b0, b1 float64

		for y := range h {
			f1, f2, f3 := fy[1*h+y], fy[2*h+y], fy[3*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
				b0 += u
				b1 += float64(u * f1)
			}
		}

		sa, sb = [maxRows]float64{a0, a1, a2, a3}, [maxRows]float64{b0, b1}
	case 4*cosStride + 3:
		var a0, a1, a2, a3, b0, b1, b2 float64

		for y := range h {
			f1, f2, f3 := fy[1*h+y], fy[2*h+y], fy[3*h+y]

			for x, v := range ch[y*w : y*w+w] {
				c := (*[cosStride]float64)(fx[x*cosStride : x*cosStride+cosStride])
				t := float64(v * c[colA])
				u := float64(v * c[colB])
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				a3 += float64(t * f3)
				b0 += u
				b1 += float64(u * f1)
				b2 += float64(u * f2)
			}
		}

		sa, sb = [maxRows]float64{a0, a1, a2, a3}, [maxRows]float64{b0, b1, b2}
	default:
		panic(fmt.Sprintf("thumbhash: no pair kernel for %d and %d rows", rowsA, rowsB))
	}

	return sa, sb
}

// sumPQ is sumColumn over the P and Q channels at once, for at most
// format.PQSide rows.
func sumPQ(p, q, fx, fy []float64, w, h, cx, rows int) (sp, sq [maxRows]float64) {
	col := cx & cosMask

	// One case per row count, as in sumColumn.
	switch rows {
	case 1:
		var a0, b0 float64

		for y := range h {
			rq := q[y*w : y*w+w]

			for x, v := range p[y*w : y*w+w] {
				f := fx[x*cosStride+col]
				t := float64(v * f)
				u := float64(rq[x] * f)
				a0 += t
				b0 += u
			}
		}

		sp, sq = [maxRows]float64{a0}, [maxRows]float64{b0}
	case 2:
		var a0, a1, b0, b1 float64

		for y := range h {
			f1 := fy[1*h+y]

			rq := q[y*w : y*w+w]

			for x, v := range p[y*w : y*w+w] {
				f := fx[x*cosStride+col]
				t := float64(v * f)
				u := float64(rq[x] * f)
				a0 += t
				a1 += float64(t * f1)
				b0 += u
				b1 += float64(u * f1)
			}
		}

		sp, sq = [maxRows]float64{a0, a1}, [maxRows]float64{b0, b1}
	case 3:
		var a0, a1, a2, b0, b1, b2 float64

		for y := range h {
			f1, f2 := fy[1*h+y], fy[2*h+y]

			rq := q[y*w : y*w+w]

			for x, v := range p[y*w : y*w+w] {
				f := fx[x*cosStride+col]
				t := float64(v * f)
				u := float64(rq[x] * f)
				a0 += t
				a1 += float64(t * f1)
				a2 += float64(t * f2)
				b0 += u
				b1 += float64(u * f1)
				b2 += float64(u * f2)
			}
		}

		sp, sq = [maxRows]float64{a0, a1, a2}, [maxRows]float64{b0, b1, b2}
	}

	return sp, sq
}
