import { workspace } from 'vscode';

export enum LogLevel {
    OFF = 0,
    ERROR = 1,
    WARN = 2,
    INFO = 3,
    DEBUG = 4,
    TRACE = 5
}

export class Logger {
    private static prefix: string = '[Craft Client]';

    public static getCurrentLevel(): LogLevel {
        const level = workspace.getConfiguration('craft').get('logging.level', 'warn');
        switch (level.toLowerCase()) {
            case 'off': return LogLevel.OFF;
            case 'error': return LogLevel.ERROR;
            case 'warn': return LogLevel.WARN;
            case 'info': return LogLevel.INFO;
            case 'debug': return LogLevel.DEBUG;
            case 'trace': return LogLevel.TRACE;
            default: return LogLevel.WARN;
        }
    }

    public static error(...args: any[]): void {
        if (this.getCurrentLevel() >= LogLevel.ERROR) {
            console.error(this.prefix, '[ERROR]', ...args);
        }
    }

    public static warn(...args: any[]): void {
        if (this.getCurrentLevel() >= LogLevel.WARN) {
            console.warn(this.prefix, '[WARN]', ...args);
        }
    }

    public static info(...args: any[]): void {
        if (this.getCurrentLevel() >= LogLevel.INFO) {
            console.info(this.prefix, '[INFO]', ...args);
        }
    }

    public static debug(...args: any[]): void {
        if (this.getCurrentLevel() >= LogLevel.DEBUG) {
            console.log(this.prefix, '[DEBUG]', ...args);
        }
    }

    public static trace(...args: any[]): void {
        if (this.getCurrentLevel() >= LogLevel.TRACE) {
            console.log(this.prefix, '[TRACE]', ...args);
        }
    }
}