"""Tests for Polish holidays module."""

from datetime import date

from forecast.src.holidays import get_holidays, is_holiday, _easter_sunday


class TestEasterSunday:
    """Verify Easter computation against known dates."""

    def test_known_years(self):
        known = {
            2020: date(2020, 4, 12),
            2021: date(2021, 4, 4),
            2022: date(2022, 4, 17),
            2023: date(2023, 4, 9),
            2024: date(2024, 3, 31),
            2025: date(2025, 4, 20),
            2026: date(2026, 4, 5),
        }
        for year, expected in known.items():
            assert _easter_sunday(year) == expected, f"Easter {year}: expected {expected}, got {_easter_sunday(year)}"


class TestGetHolidays:
    def test_count(self):
        """Poland has 12 bank holidays per year."""
        for year in (2024, 2025, 2026):
            assert len(get_holidays(year)) == 12

    def test_fixed_holidays_present(self):
        """Check all fixed holidays are in the list."""
        holidays = get_holidays(2025)
        assert date(2025, 1, 1) in holidays
        assert date(2025, 1, 6) in holidays
        assert date(2025, 5, 1) in holidays
        assert date(2025, 5, 3) in holidays
        assert date(2025, 8, 15) in holidays
        assert date(2025, 11, 1) in holidays
        assert date(2025, 11, 11) in holidays
        assert date(2025, 12, 25) in holidays
        assert date(2025, 12, 26) in holidays

    def test_moveable_holidays_2024(self):
        """Easter 2024 = March 31, so Easter Monday = April 1, Corpus Christi = May 30."""
        holidays = get_holidays(2024)
        assert date(2024, 3, 31) in holidays  # Easter
        assert date(2024, 4, 1) in holidays   # Easter Monday
        assert date(2024, 5, 30) in holidays  # Corpus Christi

    def test_sorted(self):
        """Holidays should be in chronological order."""
        holidays = get_holidays(2025)
        assert holidays == sorted(holidays)


class TestIsHoliday:
    def test_christmas(self):
        assert is_holiday(date(2025, 12, 25))

    def test_regular_day(self):
        assert not is_holiday(date(2025, 3, 12))

    def test_easter_monday_2026(self):
        assert is_holiday(date(2026, 4, 6))  # Easter 2026 = April 5, Monday = April 6
