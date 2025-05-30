grammar ArchDSL;

// Parser Rules - Basic structure for use cases
dsl: use_case* ;

use_case: 'use_case' string '{' NEWLINE* scenario* '}' NEWLINE*;

scenario: trigger action_block;

trigger: 'when' external_trigger NEWLINE+;

external_trigger: actor verb phrase?;

action_block: action*;

action: internal_action NEWLINE+;

internal_action: domain verb connector? phrase;

phrase: word+;

connector: CONNECTOR;

word: IDENTIFIER | CONNECTOR;

actor: IDENTIFIER;

domain: IDENTIFIER;

verb: IDENTIFIER;

string: STRING;

// Lexer Rules
CONNECTOR: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by';

IDENTIFIER: [a-zA-Z_][a-zA-Z0-9_-]*;

STRING: '"' (~["\r\n])* '"';

NEWLINE: '\r'? '\n';

// Whitespace
WS: [ \t]+ -> skip;

// Comments
COMMENT: '//' ~[\r\n]* -> skip;