# Finish fix01 representative contrast

Source `487c23918449aaab6ab422c5d1f47b71a1669055`. Actual Go UI and API, no response replacement.

| Mode | Heading | Body | Secondary | Placeholder | Control | Status |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Light | 13.234:1 | 5.409:1 | 4.956:1 | 5.409:1 | 4.956:1 | 4.606:1 |
| Dark | 14.101:1 | 6.575:1 | 6.575:1 | 6.893:1 | 5.412:1 | 5.895:1 |

Computed CSS foreground colors were composited through ancestor background colors and gradient endpoint extremes. Values are conservative representative measurements; no claim is made about every rendered pixel or assistive technology. Disabled controls are exempt, not counted as passing active controls. Body/secondary/placeholder/control/status require 4.5:1; large headings require 3:1.

Mechanical checks M1–M4 all passed in the report, with zero document overflow/page errors. Independent reviewer disposition remains a separate gate. Original 5b04 screenshots and report remain unchanged.
