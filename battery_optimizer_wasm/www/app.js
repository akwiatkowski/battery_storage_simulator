/**
 * Battery Simulator — WASM Frontend
 *
 * Loads hourly energy data (net load + spot price), lets the user pick
 * a date or date range plus battery parameters, calls the Rust/WASM
 * simulator, and renders dual-panel Chart.js charts:
 *   Top:    SoC traces for 3 strategies (DP optimal, arbitrage, self-consumption)
 *   Bottom: Spot price bars + net load line
 */

import init, { simulate } from "./pkg/wasm_battery.js";

// ── Global state ────────────────────────────────────────────────────────────

let allData = [];          // Array of {date, net_load_w[], price_pln_kwh[]}
let dateIndex = {};        // date string → index in allData for O(1) lookup
let availableDates = [];   // sorted date strings with data
let socChart = null;       // Chart.js instance for SoC panel
let priceChart = null;     // Chart.js instance for price/load panel
let mode = "day";          // "day" or "range"

// ── Initialization ──────────────────────────────────────────────────────────

async function startup() {
    try {
        // Load WASM module and JSON data in parallel
        await init();

        const resp = await fetch("data.json");
        if (!resp.ok) throw new Error(`Data fetch failed: ${resp.status}`);
        allData = await resp.json();

        if (allData.length === 0) throw new Error("No data in data.json");

        // Build lookup index: date string → array index
        allData.forEach((d, i) => { dateIndex[d.date] = i; });
        availableDates = allData.map(d => d.date); // already sorted chronologically

        populateDateControls();
        bindEvents();

        // Show app, hide loading message
        document.getElementById("status").classList.add("hidden");
        document.getElementById("app").classList.remove("hidden");

        runSimulation();
    } catch (e) {
        document.getElementById("status").textContent = `Error: ${e.message}`;
        console.error(e);
    }
}

// ── Date controls setup ─────────────────────────────────────────────────────

function populateDateControls() {
    const first = availableDates[0];
    const last = availableDates[availableDates.length - 1];

    // Single-day: calendar input defaulting to most recent date
    const dayInput = document.getElementById("day-select");
    dayInput.min = first;
    dayInput.max = last;
    dayInput.value = last;

    // Date range: default to last 7 days
    const rangeStart = document.getElementById("range-start");
    const rangeEnd = document.getElementById("range-end");
    rangeStart.min = first;
    rangeStart.max = last;
    rangeStart.value = availableDates[Math.max(0, availableDates.length - 7)];
    rangeEnd.min = first;
    rangeEnd.max = last;
    rangeEnd.value = last;

    updateDayNavButtons();
}

// ── Event binding ───────────────────────────────────────────────────────────

function bindEvents() {
    // Date inputs trigger re-simulation
    document.getElementById("day-select").addEventListener("change", () => {
        snapToAvailableDate();
        updateDayNavButtons();
        runSimulation();
    });
    document.getElementById("range-start").addEventListener("change", runSimulation);
    document.getElementById("range-end").addEventListener("change", runSimulation);

    // Prev/next day navigation
    document.getElementById("day-prev").addEventListener("click", () => navigateDay(-1));
    document.getElementById("day-next").addEventListener("click", () => navigateDay(+1));

    // Battery parameter sliders: update display label + re-simulate on drag
    for (const id of ["p-capacity", "p-power", "p-soc-min", "p-soc-max", "p-export"]) {
        document.getElementById(id).addEventListener("input", () => {
            updateParamDisplay();
            runSimulation();
        });
    }
}

// ── Day navigation ──────────────────────────────────────────────────────────

/**
 * If the user picks a date that has no data (gap in CSV),
 * snap to the nearest available date in the requested direction.
 */
function snapToAvailableDate() {
    const input = document.getElementById("day-select");
    const val = input.value;
    if (dateIndex[val] !== undefined) return; // exact match

    // Find nearest available date (prefer same or earlier)
    let best = availableDates[0];
    for (const d of availableDates) {
        if (d <= val) best = d;
        else break;
    }
    input.value = best;
}

/**
 * Move to the previous (-1) or next (+1) available date.
 * Skips gaps in data automatically.
 */
function navigateDay(direction) {
    const current = document.getElementById("day-select").value;
    const idx = availableDates.indexOf(current);
    const newIdx = idx + direction;

    if (newIdx >= 0 && newIdx < availableDates.length) {
        document.getElementById("day-select").value = availableDates[newIdx];
        updateDayNavButtons();
        runSimulation();
    }
}

/** Enable/disable prev/next buttons at data boundaries. */
function updateDayNavButtons() {
    const current = document.getElementById("day-select").value;
    const idx = availableDates.indexOf(current);
    document.getElementById("day-prev").disabled = idx <= 0;
    document.getElementById("day-next").disabled = idx >= availableDates.length - 1;
}

// ── Parameter display ───────────────────────────────────────────────────────

/** Sync slider values to their display labels. */
function updateParamDisplay() {
    document.getElementById("v-cap").textContent = document.getElementById("p-capacity").value;
    document.getElementById("v-pow").textContent = document.getElementById("p-power").value;
    document.getElementById("v-smin").textContent = document.getElementById("p-soc-min").value;
    document.getElementById("v-smax").textContent = document.getElementById("p-soc-max").value;
    document.getElementById("v-exp").textContent = parseFloat(document.getElementById("p-export").value).toFixed(2);
}

// ── Mode toggle (single day vs date range) ──────────────────────────────────

window.setMode = function (m) {
    mode = m;
    document.getElementById("mode-day").classList.toggle("active", m === "day");
    document.getElementById("mode-range").classList.toggle("active", m === "range");
    document.getElementById("day-controls").classList.toggle("hidden", m !== "day");
    document.getElementById("range-controls").classList.toggle("hidden", m !== "range");
    runSimulation();
};

// ── Simulation ──────────────────────────────────────────────────────────────

/** Get the day objects matching the current date selection. */
function getSelectedDays() {
    if (mode === "day") {
        const date = document.getElementById("day-select").value;
        const idx = dateIndex[date];
        return idx !== undefined ? [allData[idx]] : [];
    }
    // Range mode: filter days between start and end (inclusive)
    const start = document.getElementById("range-start").value;
    const end = document.getElementById("range-end").value;
    return allData.filter(d => d.date >= start && d.date <= end);
}

/** Read battery parameters from sliders. */
function getParams() {
    return {
        capacity_kwh: parseFloat(document.getElementById("p-capacity").value),
        max_power_w: parseFloat(document.getElementById("p-power").value),
        soc_min_pct: parseFloat(document.getElementById("p-soc-min").value),
        soc_max_pct: parseFloat(document.getElementById("p-soc-max").value),
        export_coeff: parseFloat(document.getElementById("p-export").value),
    };
}

/**
 * Core loop: serialize inputs → call WASM simulate() → parse result → update UI.
 * The WASM function runs all 3 strategies (DP optimal, heuristic, self-consumption)
 * plus no-battery baseline in a single call.
 */
function runSimulation() {
    const days = getSelectedDays();
    if (days.length === 0) return;

    const params = getParams();

    // Time the WASM call for the performance display
    const t0 = performance.now();
    const resultJson = simulate(JSON.stringify(days), JSON.stringify(params));
    const elapsed = performance.now() - t0;

    const result = JSON.parse(resultJson);
    updateStats(result);
    updateCharts(result, params);

    document.getElementById("timing").textContent =
        `${result.hours} hours | 3 strategies simulated in ${elapsed.toFixed(2)} ms (WASM)`;
}

// ── Stats cards ─────────────────────────────────────────────────────────────

/** Format savings vs no-battery baseline. */
function fmtSave(base, cost) {
    const save = base - cost;
    const pct = base !== 0 ? (save / Math.abs(base) * 100) : 0;
    const verb = save >= 0 ? "saves" : "costs extra";
    return `${verb} ${Math.abs(save).toFixed(2)} PLN (${Math.abs(pct).toFixed(1)}%)`;
}

/** Update the 4 cost/savings stat cards. */
function updateStats(result) {
    const nb = result.no_battery_cost_pln;

    document.getElementById("cost-no-batt").textContent = nb.toFixed(2);
    document.getElementById("cost-opt").textContent = result.optimal.total_cost_pln.toFixed(2);
    document.getElementById("cost-heur").textContent = result.heuristic.total_cost_pln.toFixed(2);
    document.getElementById("cost-sc").textContent = result.self_consumption.total_cost_pln.toFixed(2);

    document.getElementById("save-opt").textContent = fmtSave(nb, result.optimal.total_cost_pln);
    document.getElementById("save-heur").textContent = fmtSave(nb, result.heuristic.total_cost_pln);
    document.getElementById("save-sc").textContent = fmtSave(nb, result.self_consumption.total_cost_pln);
}

// ── Chart rendering ─────────────────────────────────────────────────────────

/**
 * Build x-axis labels: hour numbers for single day,
 * date headers + 6h ticks for multi-day ranges.
 */
function makeLabels(result) {
    const labels = [];
    const multiDay = result.dates.length > 1;

    if (!multiDay) {
        // Single day: just hour numbers 0..23
        for (let i = 0; i < result.hours; i++) labels.push(i.toString());
        return labels;
    }

    // Multi-day: show date at hour 0 of each day, "6h/12h/18h" at intervals
    const avgH = Math.round(result.hours / result.dates.length);
    for (let i = 0; i < result.hours; i++) {
        const d = Math.floor(i / avgH);
        const h = i % avgH;
        if (h === 0 && d < result.dates.length) {
            labels.push(result.dates[d]);
        } else if (h % 6 === 0) {
            labels.push(`${h}h`);
        } else {
            labels.push("");
        }
    }
    return labels;
}

/**
 * Render (or re-render) both charts:
 *   - Top: SoC traces for DP optimal (amber), arbitrage (blue), self-consumption (green)
 *          + dashed SoC min/max bounds
 *   - Bottom: spot price bars (gold) on left axis, net load line (coral) on right axis
 */
function updateCharts(result, params) {
    const labels = makeLabels(result);
    const socMin = params.capacity_kwh * params.soc_min_pct / 100;
    const socMax = params.capacity_kwh * params.soc_max_pct / 100;
    const titleSuffix = result.dates.length === 1
        ? result.dates[0]
        : `${result.dates[0]} to ${result.dates[result.dates.length - 1]}`;

    // ── Top panel: SoC traces ──
    const socData = {
        labels,
        datasets: [
            {
                label: `DP Optimal (${result.optimal.total_cost_pln.toFixed(2)} PLN)`,
                data: result.optimal.soc_kwh,
                borderColor: "#f0a050",      // warm amber
                borderWidth: 2.5,
                pointRadius: 0,
                tension: 0.2,
            },
            {
                label: `Arbitrage (${result.heuristic.total_cost_pln.toFixed(2)} PLN)`,
                data: result.heuristic.soc_kwh,
                borderColor: "#4a6fa5",      // steel blue
                borderWidth: 2,
                pointRadius: 0,
                tension: 0.2,
            },
            {
                label: `Self-Consumption (${result.self_consumption.total_cost_pln.toFixed(2)} PLN)`,
                data: result.self_consumption.soc_kwh,
                borderColor: "#5bb88a",      // teal green
                borderWidth: 2,
                pointRadius: 0,
                tension: 0.2,
            },
            {
                // Dashed SoC max bound (hidden from legend via "_" prefix)
                label: "_max",
                data: Array(result.hours).fill(socMax),
                borderColor: "rgba(150,150,150,0.4)",
                borderDash: [6, 4],
                borderWidth: 1,
                pointRadius: 0,
                fill: false,
            },
            {
                // Dashed SoC min bound
                label: "_min",
                data: Array(result.hours).fill(socMin),
                borderColor: "rgba(150,150,150,0.4)",
                borderDash: [6, 4],
                borderWidth: 1,
                pointRadius: 0,
                fill: false,
            },
        ],
    };

    if (socChart) socChart.destroy();
    socChart = new Chart(document.getElementById("chart-soc"), {
        type: "line",
        data: socData,
        options: {
            responsive: true,
            animation: false,  // instant updates when dragging sliders
            interaction: { mode: "index", intersect: false },
            plugins: {
                title: {
                    display: true,
                    text: `Battery SoC — ${titleSuffix}`,
                    font: { size: 14, weight: "bold" },
                },
                legend: {
                    // Filter out the "_min" / "_max" bound lines
                    labels: { filter: (item) => !item.text.startsWith("_") },
                },
            },
            scales: {
                x: {
                    title: { display: result.dates.length === 1, text: "Hour" },
                    ticks: { maxRotation: 0 },
                },
                y: {
                    title: { display: true, text: "SoC (kWh)" },
                    min: 0,
                    max: params.capacity_kwh * 1.05,
                },
            },
        },
    });

    // ── Bottom panel: price bars + net load line (dual y-axis) ──
    const priceData = {
        labels,
        datasets: [
            {
                type: "bar",
                label: "Spot price",
                data: result.price_pln_kwh,
                backgroundColor: "rgba(232, 184, 48, 0.35)",  // golden bars
                borderColor: "rgba(232, 184, 48, 0.6)",
                borderWidth: 1,
                yAxisID: "y-price",
                order: 2,  // draw behind the line
            },
            {
                type: "line",
                label: "Net load",
                data: result.net_load_w.map(v => v / 1000),  // W → kW
                borderColor: "#e87c6c",   // soft coral
                borderWidth: 1.5,
                pointRadius: 0,
                tension: 0.3,
                yAxisID: "y-load",
                order: 1,  // draw on top of bars
            },
        ],
    };

    if (priceChart) priceChart.destroy();
    priceChart = new Chart(document.getElementById("chart-price"), {
        type: "bar",
        data: priceData,
        options: {
            responsive: true,
            animation: false,
            interaction: { mode: "index", intersect: false },
            plugins: { legend: { position: "top" } },
            scales: {
                x: {
                    title: { display: result.dates.length === 1, text: "Hour" },
                    ticks: { maxRotation: 0 },
                },
                "y-price": {
                    type: "linear",
                    position: "left",
                    title: { display: true, text: "Price (PLN/kWh)" },
                    min: 0,
                },
                "y-load": {
                    type: "linear",
                    position: "right",
                    title: { display: true, text: "Net load (kW)" },
                    grid: { drawOnChartArea: false },  // don't overlap price grid
                },
            },
        },
    });
}

// ── Boot ─────────────────────────────────────────────────────────────────────

startup();
