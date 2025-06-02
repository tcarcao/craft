grammar ArchDSL;

// Parser Rules
dsl: (use_case NEWLINE*)* ;

use_case: 'use_case' string '{' NEWLINE* scenario* '}';

scenario: trigger action_block;

trigger: 'when' external_trigger NEWLINE+
       | 'when' quoted_event NEWLINE+
       | 'when' domain 'listens' quoted_event NEWLINE+;

external_trigger: actor verb phrase?;

action_block: action*;

action: async_action NEWLINE+
      | sync_action NEWLINE+
      | internal_action NEWLINE+;

sync_action: domain 'asks' domain connector? phrase;

async_action: domain 'notifies' quoted_event;

internal_action: domain verb connector? phrase;

phrase: word+;

connector: CONNECTOR;

word: IDENTIFIER | CONNECTOR;

actor: IDENTIFIER;

domain: domain_or_datastore;

verb: IDENTIFIER;

quoted_event: STRING;

string: STRING;

CONNECTOR: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by';

IDENTIFIER: [a-zA-Z_][a-zA-Z0-9_-]*;

STRING: '"' (~["\r\n])* '"';

NEWLINE: '\r'? '\n';

// Whitespace (skip newlines are now significant)
WS: [ \t]+ -> skip;

// Comments (optional)
COMMENT: '//' ~[\r\n]* -> skip;