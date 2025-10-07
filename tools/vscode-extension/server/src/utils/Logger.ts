export enum LogLevel {
    OFF = 0,
    ERROR = 1,
    WARN = 2,
    INFO = 3,
    DEBUG = 4,
    TRACE = 5
}

export class Logger {
    private static currentLevel: LogLevel = LogLevel.WARN;
    private static prefix: string = '[Craft]';

    public static setLevel(level: string): void {
        switch (level.toLowerCase()) {
            case 'off':
                this.currentLevel = LogLevel.OFF;
                break;
            case 'error':
                this.currentLevel = LogLevel.ERROR;
                break;
            case 'warn':
                this.currentLevel = LogLevel.WARN;
                break;
            case 'info':
                this.currentLevel = LogLevel.INFO;
                break;
            case 'debug':
                this.currentLevel = LogLevel.DEBUG;
                break;
            case 'trace':
                this.currentLevel = LogLevel.TRACE;
                break;
            default:
                this.currentLevel = LogLevel.WARN;
        }
    }

    public static getLevel(): LogLevel {
        return this.currentLevel;
    }

    public static error(...args: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.ERROR) {
            console.error(this.prefix, '[ERROR]', ...args);
        }
    }

    public static warn(...args: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.WARN) {
            console.warn(this.prefix, '[WARN]', ...args);
        }
    }

    public static info(...args: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.INFO) {
            console.info(this.prefix, '[INFO]', ...args);
        }
    }

    public static debug(...args: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.DEBUG) {
            console.log(this.prefix, '[DEBUG]', ...args);
        }
    }

    public static trace(...args: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.TRACE) {
            console.log(this.prefix, '[TRACE]', ...args);
        }
    }

    // Convenience methods for common debugging scenarios
    public static debugExtraction(operation: string, ...args: any[]): void {
        this.debug(`[DSL Extraction] ${operation}:`, ...args);
    }

    public static traceNodes(operation: string, nodes: any[]): void {
        if ((this.currentLevel ?? LogLevel.WARN) >= LogLevel.TRACE) {
            this.trace(`[AST Nodes] ${operation}:`);
            nodes.forEach((node, index) => {
                if (node && node.constructor && node.start && node.stop) {
                    this.trace(`  Node ${index}: ${node.constructor.name} at lines ${node.start.line}-${node.stop.line}`);
                } else {
                    this.trace(`  Node ${index}:`, node);
                }
            });
        }
    }

    public static debugServerRequest(method: string, args?: any): void {
        this.debug(`[Server Request] ${method}`, args ? { args } : '');
    }

    public static debugServerResponse(method: string, result: any, duration?: number): void {
        const timing = duration ? ` (${duration}ms)` : '';
        this.debug(`[Server Response] ${method}${timing}`, typeof result === 'string' ? `${result.length} chars` : result);
    }
}