# DNP QW410 / Citizen CZ-01: Incorrect 4x6 Image Width Causes Horizontal Scaling and Aliasing

## Description

Gutenprint uses incorrect horizontal margins for 4x6 output on the DNP QW410 / Citizen CZ-01.

The printer protocol requires a `1408x1836` raster. Comparison with Citizen's official macOS driver shows that the correct active image area is:

```text
x = 92..1315
width = 1224 pixels
```

Gutenprint currently defines 71-pixel margins:

```c
DEFINE_PAPER(
  "w288h432", "4x6",
  PT(1408,300), PT(1836,300),
  PT(71,300), PT(71,300), 0, 0,
  DYESUB_PORTRAIT
)
```

This produces:

```text
x = 71..1336
width = 1266 pixels
```

Consequently, Gutenprint horizontally scales the `1224x1836` CUPS raster to `1266x1836`. The scaling is anisotropic and uses `dyesub_interpolate()`, which is point/nearest-neighbor sampling. It introduces clearly visible stair-step aliasing on diagonal edges.

The official macOS driver does not perform this scaling.

## Tested Environment

```text
Printer: Citizen CZ-01
USB ID: 1343:000c
Gutenprint: 5.3.4 and current master
CUPS raster: 1224x1836, RGB, 300 dpi
Output protocol raster: 1408x1836
```

## Root Cause

The 71-pixel margins are defined internally in the Gutenprint dye-sub driver. They cannot be corrected through the PPD or standard CUPS scaling options.

## Proposed Fix

Change the 4x6 horizontal margins from 71 to 92 pixels:

```diff
-  DEFINE_PAPER( "w288h432", "4x6", PT(1408,300), PT(1836,300), PT(71,300), PT(71,300), 0, 0, DYESUB_PORTRAIT),
+  DEFINE_PAPER( "w288h432", "4x6", PT(1408,300), PT(1836,300), PT(92,300), PT(92,300), 0, 0, DYESUB_PORTRAIT),
```

This gives the correct active width:

```text
1408 - 92 - 92 = 1224
```

After applying the patch:

```text
Gutenprint:    x=92..1315, width=1224
macOS driver:  x=92..1315, width=1224
```

The unnecessary horizontal resampling is eliminated, and physical test prints no longer show the aliasing artifacts.

The change was verified both with synthetic raster patterns and real photographs on the Citizen CZ-01.

## AI Assistance Disclosure

AI tools assisted with analyzing the captured print streams and drafting this report. All measurements, comparisons, code changes, and physical print results described above were independently reproduced and verified on the actual hardware.
