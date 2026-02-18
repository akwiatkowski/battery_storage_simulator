# ============================================================================
# theme.R — Shared color palette, ggplot theme, and plot helpers
# ============================================================================
# Provides a consistent visual language across all analysis scripts.
# Matches the frontend webapp color palette defined in CLAUDE.md.
#
# Usage:
#   source("analysis/helpers/theme.R")   # or via load_data.R which sources this
#
# Exports:
#   COLORS         — named list of energy-themed hex colors
#   SEASON_COLORS  — named vector for Spring/Summer/Autumn/Winter
#   theme_energy() — clean ggplot theme extending theme_minimal()
#   scale_fill_power()  — diverging fill scale (export green ↔ import coral)
#   scale_color_power() — diverging color scale (export green ↔ import coral)
#   save_plot()    — saves a ggplot to the output/ directory
# ============================================================================

library(ggplot2)

# --- Color palette (matches frontend/src/lib/components/ conventions) --------

# COLORS is a named list so you can write COLORS$import, COLORS$pv, etc.
COLORS <- list(
  import     = "#e87c6c",   # soft coral — grid consumption

export     = "#5bb88a",   # teal green — grid export / savings
  charge     = "#64b5f6",   # light electric blue — battery charging
  discharge  = "#f0a050",   # warm amber — battery discharging
  pv         = "#e8b830",   # golden — solar PV generation
  heat_pump  = "#e8884c",   # warm orange — heat pump
  prediction = "#9b8fd8",   # soft violet — NN predictions
  warning    = "#e0a040",   # amber — warnings
  bg         = "#f7f9fc",   # page background
  border     = "#e8ecf1",   # card borders
  text       = "#333333",   # dark text
  muted      = "grey50"     # reference lines, annotations
)

# Named vector for season facets / color scales
SEASON_COLORS <- c(
  "Spring" = "#5bb88a",
  "Summer" = "#e8b830",
  "Autumn" = "#e8884c",
  "Winter" = "#64b5f6"
)

# --- ggplot theme ------------------------------------------------------------

# theme_energy() extends theme_minimal() with:
#   - light grey panel background matching the webapp
#   - bottom-positioned legends
#   - subtle grid lines
#   - consistent font sizing
theme_energy <- function(base_size = 12) {
  theme_minimal(base_size = base_size) %+replace%
    theme(
      # Panel
      panel.background  = element_rect(fill = COLORS$bg, color = NA),
      plot.background   = element_rect(fill = "white", color = NA),
      panel.grid.minor  = element_line(color = COLORS$border, linewidth = 0.3),
      panel.grid.major  = element_line(color = COLORS$border, linewidth = 0.5),
      # Legend
      legend.position   = "bottom",
      legend.background = element_rect(fill = "white", color = NA),
      # Title
      plot.title        = element_text(face = "bold", size = base_size + 2, hjust = 0),
      plot.subtitle     = element_text(color = "grey40", size = base_size, hjust = 0),
      # Axis
      axis.title        = element_text(color = "grey30"),
      axis.text         = element_text(color = "grey40")
    )
}

# --- Scale helpers -----------------------------------------------------------

# Diverging fill scale: green (export/negative) ↔ white ↔ coral (import/positive)
# Use with power data where negative = export, positive = import.
scale_fill_power <- function(midpoint = 0, ...) {
  scale_fill_gradient2(
    low = COLORS$export, mid = "white", high = COLORS$import,
    midpoint = midpoint, name = "Power (W)", ...
  )
}

# Matching color scale for lines/points
scale_color_power <- function(midpoint = 0, ...) {
  scale_color_gradient2(
    low = COLORS$export, mid = "white", high = COLORS$import,
    midpoint = midpoint, name = "Power (W)", ...
  )
}

# --- Plot saving helper ------------------------------------------------------

# save_plot() writes a ggplot object to docs/analysis/<filename>.
# Ensures the output directory exists and prints a confirmation message.
#
# Args:
#   plot     — a ggplot object
#   filename — just the filename, e.g. "01_cop_vs_temp.png"
#   width    — plot width in inches (default 10)
#   height   — plot height in inches (default 6)
save_plot <- function(plot, filename, width = 10, height = 6) {
  # Resolve output directory relative to the project root.
  # Scripts are run from the project root via `Rscript analysis/scripts/...`
  out_dir <- "docs/analysis"
  if (!dir.exists(out_dir)) dir.create(out_dir, recursive = TRUE)

  path <- file.path(out_dir, filename)
  ggsave(path, plot, width = width, height = height)
  cat("Saved:", path, "\n")
}
