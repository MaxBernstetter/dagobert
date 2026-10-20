# Moving average

One definition. SMA and EMA are `method` on that definition, not two copies of it.

```json
{"type": "MovingAverage", "method": "sma", "time_frame_hr": 3}
```

`time_frame_hr` is a positive multiple of 3. Unit: `price`.

A longer window reuses the shorter window's sum when both are on the graph. It does not walk those bars again. Close-distance and the Bollinger mid take this result.
