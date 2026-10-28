/* Configuration globals for spumux's image-only build.
 * easyDVD supplies PNG highlights and does not use spumux text subtitles.
 * SPDX-License-Identifier: MIT (compatible with the combined helper's GPL).
 */
#include "compat.h"
#include "subfont.h"
font_desc_t *vo_font = NULL;
char *sub_font = NULL;
float text_font_scale_factor = 28.0;
float subtitle_font_thickness = 3.0;
colorspec subtitle_fill_color = {255, 255, 255, 255};
colorspec subtitle_outline_color = {0, 0, 0, 255};
colorspec subtitle_shadow_color = {0, 0, 0, 255};
int subtitle_shadow_dx = 0;
int subtitle_shadow_dy = 0;
