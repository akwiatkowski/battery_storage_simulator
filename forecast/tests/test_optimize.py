"""Tests for battery schedule optimization."""

import numpy as np
import pytest

from forecast.src.optimize import (
    BatteryParams,
    optimize_battery,
    simulate_heuristic,
    simulate_no_battery,
)

# Standard test battery: 10 kWh, 5 kW, 10-90% SoC
PARAMS = BatteryParams(
    capacity_wh=10_000,
    max_power_w=5_000,
    soc_min_wh=1_000,
    soc_max_wh=9_000,
    export_coeff=0.8,
)


def make_price_spread():
    """24h prices: cheap at night (0.10), expensive during day (0.50)."""
    price = np.full(24, 0.30)
    price[:6] = 0.10   # midnight-5am: cheap
    price[17:21] = 0.50  # 5-8pm: expensive
    return price


class TestLPOptimal:
    def test_flat_price_no_benefit(self):
        """With flat prices, battery can't arbitrage -- cost should equal no-battery."""
        net_load = np.full(24, 1000.0)  # constant 1kW import
        price = np.full(24, 0.30)

        no_batt = simulate_no_battery(net_load, price, PARAMS.export_coeff)
        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert opt.status == "optimal"
        assert opt.total_cost_pln == pytest.approx(no_batt.total_cost_pln, abs=0.01)

    def test_price_spread_arbitrage(self):
        """With price spread, optimal should be cheaper than no-battery."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        no_batt = simulate_no_battery(net_load, price, PARAMS.export_coeff)
        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert opt.status == "optimal"
        assert opt.total_cost_pln < no_batt.total_cost_pln

    def test_soc_bounds(self):
        """SoC should never go below min or above max."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert opt.soc_wh.min() >= PARAMS.soc_min_wh - 1e-6
        assert opt.soc_wh.max() <= PARAMS.soc_max_wh + 1e-6

    def test_power_bounds(self):
        """Charge/discharge should not exceed max power."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert opt.charge_w.max() <= PARAMS.max_power_w + 1e-6
        assert opt.discharge_w.max() <= PARAMS.max_power_w + 1e-6

    def test_energy_balance(self):
        """Energy balance: import - export = net_load + charge - discharge."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        balance = opt.import_w - opt.export_w - net_load - opt.charge_w + opt.discharge_w
        np.testing.assert_allclose(balance, 0, atol=1e-6)

    def test_soc_evolution(self):
        """SoC[t] = SoC[t-1] + charge[t] - discharge[t]."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()
        initial_soc = PARAMS.soc_min_wh

        opt = optimize_battery(net_load, price, PARAMS, initial_soc)

        # Check first timestep
        expected_soc0 = initial_soc + opt.charge_w[0] - opt.discharge_w[0]
        assert opt.soc_wh[0] == pytest.approx(expected_soc0, abs=1e-6)

        # Check all subsequent timesteps
        for t in range(1, 24):
            expected = opt.soc_wh[t - 1] + opt.charge_w[t] - opt.discharge_w[t]
            assert opt.soc_wh[t] == pytest.approx(expected, abs=1e-6)

    def test_optimal_leq_heuristic(self):
        """LP optimal should never be more expensive than heuristic."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()
        initial_soc = PARAMS.soc_min_wh

        heur = simulate_heuristic(net_load, price, PARAMS, initial_soc)
        opt = optimize_battery(net_load, price, PARAMS, initial_soc)

        assert opt.total_cost_pln <= heur.total_cost_pln + 1e-6

    def test_charges_during_cheap_hours(self):
        """With price spread, LP should charge during cheap hours."""
        net_load = np.full(24, 500.0)
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        # Should charge more during cheap hours (0-5) than expensive hours (17-20)
        cheap_charge = opt.charge_w[:6].sum()
        expensive_charge = opt.charge_w[17:21].sum()
        assert cheap_charge > expensive_charge

    def test_export_only_loads(self):
        """With negative net load (PV surplus), battery should still work."""
        net_load = np.full(24, -2000.0)  # constant 2kW export
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert opt.status == "optimal"
        # Energy balance still holds
        balance = opt.import_w - opt.export_w - net_load - opt.charge_w + opt.discharge_w
        np.testing.assert_allclose(balance, 0, atol=1e-6)

    def test_mixed_net_load(self):
        """Mix of import and export hours."""
        net_load = np.zeros(24)
        net_load[:12] = -3000  # morning: PV surplus
        net_load[12:] = 2000   # evening: consumption
        price = make_price_spread()

        opt = optimize_battery(net_load, price, PARAMS, PARAMS.soc_min_wh)
        no_batt = simulate_no_battery(net_load, price, PARAMS.export_coeff)

        assert opt.status == "optimal"
        assert opt.total_cost_pln <= no_batt.total_cost_pln + 1e-6


class TestHeuristic:
    def test_p33_p67_indexing(self):
        """P33/P67 indexing should match Go: index = (n-1) * pct / 100."""
        # 24 prices from 0.01 to 0.24
        price = np.arange(1, 25) / 100.0

        # Go indexing: idx33 = int((24-1) * 33 / 100) = int(7.59) = 7 -> price[7] = 0.08
        #              idx67 = int((24-1) * 67 / 100) = int(15.41) = 15 -> price[15] = 0.16
        from forecast.src.optimize import _daily_percentiles
        p33, p67 = _daily_percentiles(price)

        assert p33 == pytest.approx(0.08)
        assert p67 == pytest.approx(0.16)

    def test_charges_when_cheap(self):
        """Heuristic should charge when price <= P33."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        heur = simulate_heuristic(net_load, price, PARAMS, PARAMS.soc_min_wh)

        # Cheap hours (0-5, price=0.10) should have charging
        assert heur.charge_w[:6].sum() > 0

    def test_discharges_when_expensive(self):
        """Heuristic should discharge when price >= P67."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        # Need some SoC to discharge -- start at max
        heur = simulate_heuristic(net_load, price, PARAMS, PARAMS.soc_max_wh)

        # Expensive hours (17-20, price=0.50) should have discharging
        assert heur.discharge_w[17:21].sum() > 0

    def test_soc_bounds_respected(self):
        """Heuristic should respect SoC bounds."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        heur = simulate_heuristic(net_load, price, PARAMS, PARAMS.soc_min_wh)

        assert heur.soc_wh.min() >= PARAMS.soc_min_wh - 1e-6
        assert heur.soc_wh.max() <= PARAMS.soc_max_wh + 1e-6

    def test_energy_balance(self):
        """Heuristic energy balance: import - export = net_load + charge - discharge."""
        net_load = np.full(24, 1000.0)
        price = make_price_spread()

        heur = simulate_heuristic(net_load, price, PARAMS, PARAMS.soc_min_wh)

        balance = heur.import_w - heur.export_w - net_load - heur.charge_w + heur.discharge_w
        np.testing.assert_allclose(balance, 0, atol=1e-6)


class TestNoBattery:
    def test_import_only(self):
        """Positive net load = all import."""
        net_load = np.full(24, 1000.0)
        price = np.full(24, 0.30)

        result = simulate_no_battery(net_load, price, 0.8)

        np.testing.assert_allclose(result.import_w, 1000.0)
        np.testing.assert_allclose(result.export_w, 0.0)
        assert result.total_cost_pln == pytest.approx(24 * 1000 * 0.30 / 1000)

    def test_export_only(self):
        """Negative net load = all export."""
        net_load = np.full(24, -1000.0)
        price = np.full(24, 0.30)

        result = simulate_no_battery(net_load, price, 0.8)

        np.testing.assert_allclose(result.import_w, 0.0)
        np.testing.assert_allclose(result.export_w, 1000.0)
        # Revenue = 24 * 1000 * 0.30 * 0.8 / 1000 = 5.76 PLN (negative cost)
        assert result.total_cost_pln == pytest.approx(-5.76)

    def test_mixed_load(self):
        """Mixed import/export hours."""
        net_load = np.array([1000.0, -1000.0])
        price = np.array([0.30, 0.30])

        result = simulate_no_battery(net_load, price, 0.8)

        assert result.import_w[0] == pytest.approx(1000.0)
        assert result.export_w[0] == pytest.approx(0.0)
        assert result.import_w[1] == pytest.approx(0.0)
        assert result.export_w[1] == pytest.approx(1000.0)
