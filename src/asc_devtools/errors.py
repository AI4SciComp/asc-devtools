"""Expected errors presented without tracebacks."""


class AscError(Exception):
    """Base class for user-facing failures."""


class ConfigurationError(AscError):
    """Configuration loading or validation failed."""


class ProcessError(AscError):
    """An external command could not be executed successfully."""

    def __init__(self, message: str, return_code: int = 1) -> None:
        super().__init__(message)
        self.return_code = return_code


class RepositoryError(AscError):
    """A repository operation was invalid or unsafe."""
