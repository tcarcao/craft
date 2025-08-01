grammar ArchDSL;

// Parser Rules
dsl: services? (use_case NEWLINE*)* ;

services: 'services' '{' NEWLINE* service_definition_list? '}' NEWLINE*;

service_definition_list: service_definition (',' NEWLINE* service_definition)* ','? NEWLINE*;

service_definition: service_name ':' '{' NEWLINE* service_properties NEWLINE* '}' NEWLINE*;

service_name: IDENTIFIER | STRING;

service_properties: service_property (NEWLINE+ service_property)* NEWLINE*;

service_property: DOMAINS ':' domain_list
                | DATA_STORES ':' datastore_list
                | LANGUAGE ':' IDENTIFIER
                ;

domain_list: domain_or_datastore (',' domain_or_datastore)* ','?;

datastore_list: domain_or_datastore (',' domain_or_datastore)* ','?;

domain_or_datastore: IDENTIFIER;

datastore: domain_or_datastore;

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

// Lexer Rules (specific tokens that could conflict with IDENTIFIER)
DOMAINS: 'domains';
DATA_STORES: 'data-stores';
LANGUAGE: 'language';

CONNECTOR: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by';

IDENTIFIER: [a-zA-Z_][a-zA-Z0-9_-]*;

STRING: '"' (~["\r\n])* '"';

NEWLINE: '\r'? '\n';

// Whitespace (skip newlines are now significant)
WS: [ \t]+ -> skip;

// Comments (optional)
COMMENT: '//' ~[\r\n]* -> skip;