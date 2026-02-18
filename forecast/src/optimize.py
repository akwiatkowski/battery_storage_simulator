"""Battery schedule optimization: LP optimal vs P33/P67 heuristic vs no-battery."""

from dataclasses import dataclass

import numpy as np
import pandas as pd
from scipy import sparse
from scipy.optimize import linprog


@dataclass
class BatteryParams:
    capacity_wh: float
    max_power_w: float
    soc_min_wh: float
    soc_max_wh: float
    export_coeff: float = 0.8


@dataclass
class OptimizeResult:
    charge_w: np.ndarray
    discharge_w: np.ndarray
    import_w: np.ndarray
    export_w: np.ndarray
    soc_wh: np.ndarray
    cost_pln: np.ndarray
    total_cost_pln: float
    status: str


def optimize_battery(
    net_load_w: np.ndarray,
    price: np.ndarray,
    params: BatteryParams,
    initial_soc_wh: float,
) -> OptimizeResult:
    """Solve for the cost-minimizing battery schedule using linear programming.

    Variables: x = [charge(T), discharge(T), import(T), export(T), soc(T)]
    """
    T = len(net_load_w)
    N = 5 * T  # total variables

    # Objective: minimize sum(import * price - export * price * export_coeff) / 1000
    c = np.zeros(N)
    c[2 * T : 3 * T] = price / 1000.0  # import cost
    c[3 * T : 4 * T] = -price * params.export_coeff / 1000.0  # export revenue

    # Equality constraints: A_eq @ x = b_eq
    # 2T rows: T energy balance + T SoC evolution
    A_eq = sparse.lil_matrix((2 * T, N))
    b_eq = np.zeros(2 * T)

    for t in range(T):
        # Energy balance: import[t] - export[t] - charge[t] + discharge[t] = net_load[t]
        row = t
        A_eq[row, t] = -1.0           # charge
        A_eq[row, T + t] = 1.0        # discharge
        A_eq[row, 2 * T + t] = 1.0    # import
        A_eq[row, 3 * T + t] = -1.0   # export
        b_eq[row] = net_load_w[t]

        # SoC evolution: soc[t] - charge[t] + discharge[t] = soc[t-1]
        row = T + t
        A_eq[row, 4 * T + t] = 1.0    # soc[t]
        A_eq[row, t] = -1.0           # charge
        A_eq[row, T + t] = 1.0        # discharge
        if t == 0:
            b_eq[row] = initial_soc_wh
        else:
            A_eq[row, 4 * T + t - 1] = -1.0  # -soc[t-1]

    A_eq = A_eq.tocsc()

    # Variable bounds
    bounds = []
    for _t in range(T):
        bounds.append((0, params.max_power_w))   # charge
    for _t in range(T):
        bounds.append((0, params.max_power_w))   # discharge
    for _t in range(T):
        bounds.append((0, None))                 # import
    for _t in range(T):
        bounds.append((0, None))                 # export
    for _t in range(T):
        bounds.append((params.soc_min_wh, params.soc_max_wh))  # soc

    result = linprog(c, A_eq=A_eq, b_eq=b_eq, bounds=bounds, method="highs")

    if not result.success:
        return OptimizeResult(
            charge_w=np.zeros(T),
            discharge_w=np.zeros(T),
            import_w=np.maximum(net_load_w, 0),
            export_w=np.maximum(-net_load_w, 0),
            soc_wh=np.full(T, initial_soc_wh),
            cost_pln=np.zeros(T),
            total_cost_pln=0.0,
            status=result.message,
        )

    x = result.x
    charge = x[:T]
    discharge = x[T : 2 * T]
    imp = x[2 * T : 3 * T]
    exp = x[3 * T : 4 * T]
    soc = x[4 * T : 5 * T]
    cost = (imp * price - exp * price * params.export_coeff) / 1000.0

    return OptimizeResult(
        charge_w=charge,
        discharge_w=discharge,
        import_w=imp,
        export_w=exp,
        soc_wh=soc,
        cost_pln=cost,
        total_cost_pln=float(cost.sum()),
        status="optimal",
    )


def simulate_heuristic(
    net_load_w: np.ndarray,
    price: np.ndarray,
    params: BatteryParams,
    initial_soc_wh: float,
) -> OptimizeResult:
    """Simulate P33/P67 daily percentile heuristic matching Go implementation.

    Charges at max power when price <= P33, discharges at max when >= P67.
    Uses Go-compatible percentile indexing: index = (n-1) * pct / 100.
    """
    T = len(net_load_w)
    charge = np.zeros(T)
    discharge = np.zeros(T)
    imp = np.zeros(T)
    exp = np.zeros(T)
    soc = np.zeros(T)

    # Compute daily P33/P67 thresholds
    p33, p67 = _daily_percentiles(price)

    current_soc = initial_soc_wh

    for t in range(T):
        p = price[t]

        # Determine charge/discharge action
        if p <= p33:
            # Charge at max power (can import from grid)
            charge_power = min(params.max_power_w, params.soc_max_wh - current_soc)
            charge_power = max(0.0, charge_power)
            charge[t] = charge_power
        elif p >= p67:
            # Discharge at max power
            discharge_power = min(params.max_power_w, current_soc - params.soc_min_wh)
            discharge_power = max(0.0, discharge_power)
            discharge[t] = discharge_power

        # Update SoC
        current_soc = current_soc + charge[t] - discharge[t]
        soc[t] = current_soc

        # Grid flows: net_load + charge - discharge
        net = net_load_w[t] + charge[t] - discharge[t]
        if net >= 0:
            imp[t] = net
        else:
            exp[t] = -net

    cost = (imp * price - exp * price * params.export_coeff) / 1000.0

    return OptimizeResult(
        charge_w=charge,
        discharge_w=discharge,
        import_w=imp,
        export_w=exp,
        soc_wh=soc,
        cost_pln=cost,
        total_cost_pln=float(cost.sum()),
        status="heuristic",
    )


def _daily_percentiles(price: np.ndarray) -> tuple[float, float]:
    """Compute P33/P67 using Go-compatible indexing: index = (n-1) * pct / 100."""
    sorted_prices = np.sort(price)
    n = len(sorted_prices)
    if n == 0:
        return 0.0, 0.0

    idx33 = int((n - 1) * 33 / 100)
    idx67 = int((n - 1) * 67 / 100)
    return float(sorted_prices[idx33]), float(sorted_prices[idx67])


def simulate_no_battery(
    net_load_w: np.ndarray,
    price: np.ndarray,
    export_coeff: float,
) -> OptimizeResult:
    """Compute cost without any battery -- direct grid import/export."""
    T = len(net_load_w)
    imp = np.maximum(net_load_w, 0.0)
    exp = np.maximum(-net_load_w, 0.0)
    cost = (imp * price - exp * price * export_coeff) / 1000.0

    return OptimizeResult(
        charge_w=np.zeros(T),
        discharge_w=np.zeros(T),
        import_w=imp,
        export_w=exp,
        soc_wh=np.zeros(T),
        cost_pln=cost,
        total_cost_pln=float(cost.sum()),
        status="no_battery",
    )


def prepare_hourly_data(
    grid_power_df: pd.DataFrame,
    spot_prices_df: pd.DataFrame,
) -> pd.DataFrame:
    """Align grid power and spot prices to a common hourly UTC index.

    Input DataFrames must have columns: timestamp (tz-aware), value (float).
    Returns DataFrame with columns: net_load_w, price_pln_kwh, indexed by UTC hour.
    """
    gp = grid_power_df.set_index("timestamp").copy()
    gp.index = gp.index.tz_convert("UTC")
    gp_hourly = gp["value"].resample("h").mean().rename("net_load_w")

    sp = spot_prices_df.set_index("timestamp").copy()
    if sp.index.tz is None:
        sp.index = sp.index.tz_localize("UTC")
    else:
        sp.index = sp.index.tz_convert("UTC")
    sp_hourly = sp["value"].resample("h").mean().rename("price_pln_kwh")

    combined = pd.concat([gp_hourly, sp_hourly], axis=1).dropna()
    return combined
