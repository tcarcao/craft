grammar ArchDSL;

dsl: (arch | services | service_def | exposure | use_case | domain_def | domains_def)* ;

// Domain hierarchy definitions
domain_def: 'domain' domain_name '{' NEWLINE* subdomain_list '}' NEWLINE*;

domains_def: 'domains' '{' NEWLINE* domain_block_list '}' NEWLINE*;

domain_block_list: domain_block (NEWLINE+ domain_block)* NEWLINE*;

domain_block: domain_name '{' NEWLINE* subdomain_list '}';

domain_name: IDENTIFIER;

subdomain_list: subdomain (NEWLINE+ subdomain)* NEWLINE*;

subdomain: IDENTIFIER;

// Architecture blocks
arch: 'arch' arch_name? '{' NEWLINE* arch_sections '}' NEWLINE*;

arch_name: IDENTIFIER;

arch_sections: (presentation_section | gateway_section)+;

presentation_section: 'presentation' ':' NEWLINE* arch_component_list NEWLINE+;

gateway_section: 'gateway' ':' NEWLINE* arch_component_list NEWLINE+;

arch_component_list: arch_component (NEWLINE+ arch_component)*;

arch_component: simple_component | component_flow;

component_flow: component_chain;

component_chain: component_with_modifiers ('>' component_with_modifiers)*;

component_with_modifiers: component_name component_modifiers?;

component_name: IDENTIFIER;

component_modifiers: '[' modifier_list ']';

modifier_list: modifier (',' modifier)*;

modifier: IDENTIFIER (':' IDENTIFIER)?;

simple_component: component_with_modifiers;

// Exposure blocks
exposure: 'exposure' exposure_name '{' NEWLINE+ exposure_properties '}' NEWLINE*;

exposure_name: IDENTIFIER;

exposure_properties: exposure_property (NEWLINE+ exposure_property)* NEWLINE+;

exposure_property: 'to' ':' target_list
                 | 'of' ':' domain_list
                 | 'through' ':' gateway_list;

target_list: target (',' target)* ','?;

target: IDENTIFIER;

gateway_list: gateway (',' gateway)* ','?;

gateway: IDENTIFIER;

// Enhanced services (keeping existing + adding deployment)
services: 'services' '{' NEWLINE* service_definition_list? '}' NEWLINE*;

// Single service definition
service_def: 'service' service_name ':' '{' NEWLINE* service_properties '}' NEWLINE*;

service_definition_list: service_definition (',' NEWLINE* service_definition)* ','? NEWLINE*;

service_definition: service_name ':' '{' NEWLINE* service_properties '}' NEWLINE*;

service_name: IDENTIFIER | STRING;

service_properties: service_property (NEWLINE+ service_property)* NEWLINE*;

service_property: DOMAINS ':' domain_list
                | DATA_STORES ':' datastore_list
                | LANGUAGE ':' IDENTIFIER
                | DEPLOYMENT ':' deployment_strategy
                ;

deployment_strategy: deployment_type ('(' deployment_config ')')?;

deployment_type: 'canary' | 'blue_green' | 'rolling';

deployment_config: deployment_rule (',' deployment_rule)*;

deployment_rule: PERCENTAGE '->' deployment_target;

deployment_target: IDENTIFIER;

domain_list: domain_ref (',' domain_ref)* ','?;

domain_ref: IDENTIFIER;

datastore_list: datastore (',' datastore)* ','?;

datastore: IDENTIFIER;

// Use case blocks
use_case: 'use_case' string '{' NEWLINE* scenario* '}' NEWLINE*;

scenario: trigger action_block;

trigger: 'when' external_trigger NEWLINE+
       | 'when' quoted_event NEWLINE+
       | 'when' domain 'listens' quoted_event NEWLINE+;

external_trigger: actor verb phrase?;

action_block: action*;

action: async_action NEWLINE+
      | sync_action NEWLINE+
      | internal_action NEWLINE+;

sync_action : domain 'asks' domain connector_word phrase
            | domain 'asks' domain phrase;

async_action: domain 'notifies' quoted_event;

internal_action: domain verb connector_word? phrase;

phrase: (IDENTIFIER | STRING | connector_word)+;

connector_word: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by';

actor: IDENTIFIER;

domain: IDENTIFIER;

verb: IDENTIFIER;

quoted_event: STRING;

string: STRING;

// Lexer Rules
DOMAINS: 'domains';
DATA_STORES: 'data-stores';
LANGUAGE: 'language';
DEPLOYMENT: 'deployment';

PERCENTAGE: [0-9]+ '%';

IDENTIFIER: [a-zA-Z0-9_][a-zA-Z0-9_.-]*;

STRING: '"' (~["\r\n])* '"';

NEWLINE: '\r'? '\n';

// Whitespace (skip newlines are now significant)
WS: [ \t]+ -> skip;

// Comments (optional)
COMMENT: '//' ~[\r\n]* -> skip;