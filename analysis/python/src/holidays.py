"""Polish bank holidays. Pure computation, no external dependencies."""

from datetime import date, timedelta


def _easter_sunday(year: int) -> date:
    """Compute Easter Sunday using the Anonymous Gregorian algorithm."""
    a = year % 19
    b, c = divmod(year, 100)
    d, e = divmod(b, 4)
    f = (b + 8) // 25
    g = (b - f + 1) // 3
    h = (19 * a + b - d - g + 15) % 30
    i, k = divmod(c, 4)
    l = (32 + 2 * e + 2 * i - h - k) % 7
    m = (a + 11 * h + 22 * l) // 451
    month, day = divmod(h + l - 7 * m + 114, 31)
    return date(year, month, day + 1)


def get_holidays(year: int) -> list[date]:
    """Return all Polish bank holidays for a given year."""
    easter = _easter_sunday(year)
    return sorted([
        date(year, 1, 1),     # New Year's Day
        date(year, 1, 6),     # Epiphany
        date(year, 5, 1),     # Labour Day
        date(year, 5, 3),     # Constitution Day
        date(year, 8, 15),    # Assumption of Mary
        date(year, 11, 1),    # All Saints' Day
        date(year, 11, 11),   # Independence Day
        date(year, 12, 25),   # Christmas Day
        date(year, 12, 26),   # Second Day of Christmas
        easter,               # Easter Sunday
        easter + timedelta(days=1),   # Easter Monday
        easter + timedelta(days=60),  # Corpus Christi
    ])


def is_holiday(d: date) -> bool:
    """Check if a date is a Polish bank holiday."""
    return d in get_holidays(d.year)
