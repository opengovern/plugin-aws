package shared

import (
	"github.com/golang/protobuf/ptypes/timestamp"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"time"
)

func WrappedToInt32(v *wrapperspb.Int32Value) *int32 {
	if v == nil {
		return nil
	}
	tmp := v.GetValue()
	return &tmp
}

func WrappedToFloat64(v *wrapperspb.DoubleValue) *float64 {
	if v == nil {
		return nil
	}
	tmp := v.GetValue()
	return &tmp
}

func WrappedToString(v *wrapperspb.StringValue) *string {
	if v == nil {
		return nil
	}
	tmp := v.GetValue()
	return &tmp
}

func Int32ToWrapper(v *int32) *wrapperspb.Int32Value {
	if v == nil {
		return nil
	}
	return wrapperspb.Int32(*v)
}

func Float64ToWrapper(v *float64) *wrapperspb.DoubleValue {
	if v == nil {
		return nil
	}
	return wrapperspb.Double(*v)
}

func StringToWrapper(v *string) *wrapperspb.StringValue {
	if v == nil {
		return nil
	}
	return wrapperspb.String(*v)
}

func TimeToTimestamp(t *time.Time) *timestamp.Timestamp {
	if t == nil {
		return nil
	}
	return &timestamp.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}
