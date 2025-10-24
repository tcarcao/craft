grammar Craft;

dsl: NEWLINE* (arch | services_def | service_def | exposure | use_case | domain_def | domains_def | actors_def | actor_def)* ;

// Domain hierarchy definitions
domain_def: 'domain' domain_name '{' NEWLINE* subdomain_list '}' NEWLINE*;

domains_def: 'domains' '{' NEWLINE* domain_block_list '}' NEWLINE*;

domain_block_list: domain_block (NEWLINE+ domain_block)* NEWLINE*;

domain_block: domain_name '{' NEWLINE* subdomain_list '}';

domain_name: identifier;

subdomain_list: subdomain (NEWLINE+ subdomain)* NEWLINE*;

subdomain: identifier;

// Actor definitions - similar pattern to domains
actor_def: 'actor' actorType actor_name NEWLINE*;

actors_def: 'actors' '{' NEWLINE* actor_definition_list '}' NEWLINE*;

actor_definition_list: actor_definition (NEWLINE+ actor_definition)* NEWLINE*;

actor_definition: actorType actor_name;

actorType: 'user' | 'system' | 'service';

actor_name: identifier;

// Architecture blocks
arch: 'arch' arch_name? '{' NEWLINE* arch_sections '}' NEWLINE*;

arch_name: identifier;

arch_sections: (presentation_section | gateway_section)+;

presentation_section: 'presentation' ':' NEWLINE* arch_component_list NEWLINE+;

gateway_section: 'gateway' ':' NEWLINE* arch_component_list NEWLINE+;

arch_component_list: arch_component (NEWLINE+ arch_component)*;

arch_component: simple_component | component_flow;

component_flow: component_chain;

component_chain: component_with_modifiers ('>' component_with_modifiers)*;

component_with_modifiers: component_name component_modifiers?;

component_name: identifier;

component_modifiers: '[' modifier_list ']';

modifier_list: modifier (',' modifier)*;

modifier: identifier (':' identifier)?;

simple_component: component_with_modifiers;

// Exposure blocks
exposure: 'exposure' exposure_name '{' NEWLINE+ exposure_properties '}' NEWLINE*;

exposure_name: identifier;

exposure_properties: exposure_property (NEWLINE+ exposure_property)* NEWLINE+;

exposure_property: 'to' ':' target_list
                 | 'of' ':' domain_list
                 | 'through' ':' gateway_list;

target_list: target (',' target)* ','?;

target: identifier;

gateway_list: gateway (',' gateway)* ','?;

gateway: identifier;

// Single service definition
service_def: 'service' service_name '{' NEWLINE* service_properties '}' NEWLINE*;

// Multiple services definition
services_def: 'services' '{' NEWLINE* service_block_list? '}' NEWLINE*;

service_block_list: service_block (NEWLINE+ service_block)* NEWLINE*;

service_block: service_name '{' NEWLINE* service_properties '}' NEWLINE*;

service_name: identifier | STRING;

service_properties: service_property (NEWLINE+ service_property)* NEWLINE*;

service_property: DOMAINS ':' domain_list
                | DATA_STORES ':' datastore_list
                | LANGUAGE ':' identifier
                | DEPLOYMENT ':' deployment_strategy
                ;

deployment_strategy: deployment_type ('(' deployment_config ')')?;

deployment_type: 'canary' | 'blue_green' | 'rolling';

deployment_config: deployment_rule (',' deployment_rule)*;

deployment_rule: PERCENTAGE '->' deployment_target;

deployment_target: identifier;

domain_list: domain_ref (',' domain_ref)* ','?;

domain_ref: identifier;

datastore_list: datastore (',' datastore)* ','?;

datastore: identifier;

// Use case blocks
use_case: 'use_case' string '{' NEWLINE* scenario* '}' NEWLINE*;

scenario: trigger action_block;

trigger: 'when' domain 'listens' quoted_event NEWLINE+
       | 'when' external_trigger NEWLINE+
       | 'when' quoted_event NEWLINE+;

external_trigger: actor verb connector_word? phrase?;

action_block: action*;

action: async_action NEWLINE+
      | sync_action NEWLINE+
      | return_action NEWLINE+
      | internal_action NEWLINE+;

sync_action : domain 'asks' domain connector_word phrase
            | domain 'asks' domain phrase;

async_action: domain 'notifies' quoted_event;

internal_action: domain verb connector_word? phrase;

return_action: domain 'returns' 'to' domain connector_word? phrase
            | domain 'returns' connector_word? phrase;

phrase: (phrase_word | STRING)+;

phrase_word: identifier
           | connector_word
           | 'when'
           | 'use_case'
           ;

connector_word: 'a' | 'an' | 'the' | 'as' | 'to' | 'from' | 'in' | 'on' | 'at' | 'for' | 'with' | 'by';

actor: identifier;

domain: identifier;

verb: identifier;



identifier: IDENTIFIER
          | 'actor'
          | 'user'
          | 'system'
          | 'service'
          | 'arch'
          | 'presentation'
          | 'gateway'
          | 'domain'
          | 'domains'
          | 'actors'
          | 'exposure'
          | 'to'
          | 'of'
          | 'through'
          | 'services'
          | 'canary'
          | 'blue_green'
          | 'rolling'
          | 'listens'
          | 'asks'
          | 'notifies'
          | 'returns'
          | 'a'
          | 'an'
          | 'the'
          | 'as'
          | 'from'
          | 'in'
          | 'on'
          | 'at'
          | 'for'
          | 'with'
          | 'by'
          | DOMAINS      // 'domains' token
          | DATA_STORES  // 'data-stores' token
          | LANGUAGE     // 'language' token
          | DEPLOYMENT   // 'deployment' token
          ;

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