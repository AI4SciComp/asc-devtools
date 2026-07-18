"""Project-specific exceptions presented by the command-line interface."""


class AscError(Exception):
    """Base class for expected, user-facing errors."""


class ConfigurationError(AscError):
    """Raised when configuration cannot be loaded or validated."""


class ProcessError(AscError):
    """Raised when an external command cannot be executed successfully."""


class RepositoryError(AscError):
    """Raised for invalid or unsafe repository operations."""
