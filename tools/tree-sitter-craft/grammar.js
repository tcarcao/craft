// Tree-sitter grammar for Craft DSL - Hybrid of working backup + ANTLR mapping
export default grammar({
  name: 'craft',

  extras: $ => [
    /[ \t]/,  // Only spaces and tabs, NOT newlines
    $.comment,
  ],

  rules: {
    // Use working backup structure for source_file
    source_file: $ => seq(
      repeat($._newline),
      repeat($._top_level_item),
    ),

    // Map ANTLR top level items to working backup names
    _top_level_item: $ => choice(
      $.arch_block,      // maps to ANTLR 'arch'
      $.services_block,  // maps to ANTLR 'services_def' 
      $.service_block,   // maps to ANTLR 'service_def'
      $.domain_block,    // maps to ANTLR 'domain_def'
      $.domains_block,   // maps to ANTLR 'domains_def'
      $.actors_block,    // maps to ANTLR 'actors_def'
      $.actor_block,     // maps to ANTLR 'actor_def'
      $.exposure_block,  // maps to ANTLR 'exposure'
      $.use_case_block,  // maps to ANTLR 'use_case'
    ),

    // Architecture - use working backup structure with ANTLR precision
    arch_block: $ => seq(
      'arch',
      optional($.identifier), // ANTLR: arch_name?
      '{',
      repeat1($._newline),
      $.arch_section,
      '}',
      repeat($._newline),
    ),

    arch_section: $ => choice(
      $.presentation_section,
      $.gateway_section,
    ),

    arch_continuation: $ => choice(
      seq(repeat1($._newline), $.arch_section),
      seq(repeat1($._newline), $.arch_component_list),
      repeat1($._newline),
    ),

    // ANTLR: presentation_section: 'presentation' ':' NEWLINE* arch_component_list NEWLINE+;
    // Let arch_component_list handle its own trailing newlines
    presentation_section: $ => seq(
      'presentation',
      ':',
      repeat($._newline),
      $.arch_component_list,
    ),

    // ANTLR: gateway_section: 'gateway' ':' NEWLINE* arch_component_list NEWLINE+;
    // Let arch_component_list handle its own trailing newlines
    gateway_section: $ => seq(
      'gateway',
      ':',
      repeat($._newline),
      $.arch_component_list,
    ),

    // ANTLR: arch_component_list: arch_component (NEWLINE+ arch_component)*;
    // Use right precedence to prevent over-consuming
    arch_component_list: $ => prec.left(seq(
      $.arch_component,
      optional($.arch_continuation),
    )),

    // ANTLR: arch_component: simple_component | component_flow;
    arch_component: $ => choice(
      $.simple_component,
      $.component_flow,
    ),

    // ANTLR: component_flow: component_chain; component_chain: component_with_modifiers ('>' component_with_modifiers)*;
    component_flow: $ => seq(
      $.component_with_modifiers,
      repeat1(seq('>', $.component_with_modifiers)),
    ),

    // ANTLR: component_with_modifiers: component_name component_modifiers?;
    // Hybrid approach: use specific node type for component names
    component_with_modifiers: $ => seq(
      $.component_name,  // specific node type for direct parent detection
      optional($.component_modifiers),
    ),

    // Specific node type for arch component names (hybrid approach)
    component_name: $ => $.identifier,

    // ANTLR: component_modifiers: '[' modifier_list ']';
    component_modifiers: $ => seq(
      '[',
      $.modifier_list,
      ']',
    ),

    // ANTLR: modifier_list: modifier (',' modifier)*;
    // Use backup's working sep1 function
    modifier_list: $ => sep1($.modifier, ','),

    // ANTLR: modifier: IDENTIFIER (':' IDENTIFIER)?;
    // Hybrid approach: use specific node types for modifier keys and values
    modifier: $ => seq(
      $.modifier_key,
      optional(seq(':', $.modifier_value)),
    ),

    // Specific node types for modifiers (hybrid approach)
    modifier_key: $ => $.identifier,
    modifier_value: $ => choice($.identifier, $.number, $.boolean),

    // ANTLR: simple_component: component_with_modifiers;
    simple_component: $ => $.component_with_modifiers,

    // Services - use working backup structure with ANTLR names mapped
    // ANTLR: services_def: 'services' '{' NEWLINE* service_block_list? '}' NEWLINE*;
    services_block: $ => seq(
      'services',
      '{',
      repeat($._newline),
      repeat(seq($.service_definition, optional($._newline))), // service_definition = service_block in ANTLR
      '}',
      repeat($._newline),
    ),

    // ANTLR: service_def: 'service' service_name '{' NEWLINE* service_properties '}' NEWLINE*;
    service_block: $ => seq(
      'service',
      $.identifier,
      '{',
      optional($._newline),
      repeat(seq($.service_property, optional($._newline))),
      '}',
      repeat($._newline),
    ),

    // ANTLR service_block mapped to service_definition
    service_definition: $ => seq(
      choice($.string, $.identifier), // service_name can be IDENTIFIER | STRING
      '{',
      repeat($._newline),
      repeat(seq($.service_property, optional($._newline))),
      '}',
    ),

    // ANTLR: service_property with correct property names
    service_property: $ => choice(
      $.domains_property,
      $.language_property,
      $.data_stores_property,
      $.deployment_property,
    ),

    domains_property: $ => seq(
      'domains',
      ':',
      $.identifier_list,
    ),

    language_property: $ => seq(
      'language',
      ':',
      $.identifier,
    ),

    data_stores_property: $ => seq(
      'data-stores',
      ':',
      $.identifier_list,
    ),

    deployment_property: $ => seq(
      'deployment',
      ':',
      $.deployment_spec,
    ),

    deployment_spec: $ => choice(
      $.deployment_type,
      seq($.deployment_type, $.deployment_rules),
    ),

    deployment_type: $ => choice('canary', 'blue_green', 'rolling'),

    deployment_rules: $ => seq(
      '(',
      sep1($.deployment_rule, '->'),
      ')',
    ),

    deployment_rule: $ => seq(
      $.percentage,
      $.identifier,
    ),

    // Domain definitions - ANTLR mapping
    // ANTLR: domain_def: 'domain' domain_name '{' NEWLINE* subdomain_list '}' NEWLINE*;
    domain_block: $ => seq(
      'domain',
      $.identifier, // domain_name = IDENTIFIER
      '{',
      optional($._newline),
      repeat(seq($.subdomain, optional($._newline))),
      '}',
      repeat($._newline),
    ),

    // ANTLR: domains_def: 'domains' '{' NEWLINE* domain_block_list '}' NEWLINE*;
    domains_block: $ => seq(
      'domains',
      '{',
      optional($._newline),
      repeat(seq($.domain_definition, optional($._newline))),
      '}',
      repeat($._newline),
    ),

    domain_definition: $ => seq(
      $.identifier,
      '{',
      optional($._newline),
      repeat(seq($.subdomain, optional($._newline))),
      '}',
    ),

    subdomain: $ => $.identifier,

    // Actors definitions - following same pattern as domains/services
    // ANTLR: actor_def: 'actor' actor_type actor_name NEWLINE*;
    actor_block: $ => seq(
      'actor',
      $.actor_type,
      $.actor_name,
      repeat($._newline),
    ),

    // ANTLR: actors_def: 'actors' '{' NEWLINE* actor_definition_list '}' NEWLINE*;  
    actors_block: $ => seq(
      'actors',
      '{',
      optional($._newline),
      repeat(seq($.actor_definition, optional($._newline))),
      '}',
      repeat($._newline),
    ),

    // ANTLR: actor_definition: actor_type actor_name;
    actor_definition: $ => seq(
      $.actor_type,
      $.actor_name,
    ),

    // ANTLR: actor_type: 'user' | 'system' | 'service';
    actor_type: $ => choice('user', 'system', 'service'),

    // ANTLR: actor_name: IDENTIFIER;
    actor_name: $ => $.identifier,

    // Exposure blocks - ANTLR mapping
    exposure_block: $ => seq(
      'exposure',
      $.identifier,
      '{',
      optional($._newline),
      repeat(seq($.exposure_property, optional($._newline))),
      '}',
      repeat($._newline),
    ),

    exposure_property: $ => choice(
      $.to_property,
      $.through_property, 
      $.of_property,
    ),

    to_property: $ => seq(
      'to',
      ':',
      $.identifier_list,
    ),

    through_property: $ => seq(
      'through',
      ':',
      $.identifier_list,
    ),

    of_property: $ => seq(
      'of',
      ':',
      $.identifier_list,
    ),

    use_case_block: $ => seq(
      'use_case',
      $.string,
      '{',
      repeat1($._newline),
      $.scenario,
      '}',
      repeat($._newline),
    ),

    scenario_continuation: $ => choice(
      seq(repeat1($._newline), $.scenario),
      seq(repeat1($._newline), $.action),
      repeat1($._newline),
    ),

    scenario: $ => seq(
      $.when_clause,
      optional($.scenario_continuation),
    ),

    when_clause: $ => seq(
      'when',
      choice(
        seq($.external_trigger, $._newline),
        seq($.event_trigger, $._newline),
        seq($.domain_listener, $._newline),
        seq($.cron_trigger, $._newline),
      ),
      repeat(seq($.action, $._newline)),
    ),

    // Hybrid approach: use specific node types for external triggers
    external_trigger: $ => prec.left(2, choice(
      seq(
        $.trigger_actor, // Actor
        $.trigger_verb, // Verb
        $.connector_word,
        $.phrase, // Phrase
      ),
      seq(
        $.trigger_actor, // Actor
        $.trigger_verb, // Verb
        $.phrase, // Phrase
      ),
      seq(
        $.trigger_actor, // Actor
        $.trigger_verb, // Verb
      )
    )),

    // Specific node types for triggers (hybrid approach)
    trigger_actor: $ => $.identifier,   // Actor name in triggers
    trigger_verb: $ => $.identifier,    // Verb in triggers

    event_trigger: $ => $.string,

    domain_listener: $ => seq(
      $.identifier, // Domain
      'listens',
      $.string, // Event
    ),

    cron_trigger: $ => prec.left(seq(
      'CRON',
      optional($.phrase),
    )),

    action: $ => choice(
      $.sync_action,
      $.async_action,
      $.internal_action,
      $.return_action,
    ),

    // Hybrid approach: use specific node types for action contexts
    sync_action: $ => prec.left(2, choice(
      seq(
        $.action_subject, // Source domain  
        'asks',
        $.action_target, // Target domain
        $.connector_word,
        $.phrase,
      ),
      seq(
        $.action_subject, // Source domain
        'asks', 
        $.action_target, // Target domain
        $.phrase,
      ),
    )),

    async_action: $ => seq(
      $.action_subject, // Domain
      'notifies',
      $.string, // Event
    ),

    // Hybrid approach: use specific node types for internal actions
    internal_action: $ => prec.left(1, seq(
      $.action_subject, // Domain/Service
      $.action_verb, // Verb
      optional($.connector_word),
      $.phrase,
    )),

    // Return action: domain returns phrase [to domain]
    return_action: $ => prec.left(2, choice(
      seq(
        $.action_subject, // Source domain
        'returns',
        optional($.connector_word),
        $.phrase,
        'to', // Specifically 'to' for returning to a domain
        $.action_target, // Target domain
      ),
      seq(
        $.action_subject, // Source domain
        'returns',
        optional($.connector_word),
        $.phrase,
      ),
    )),

    // Specific node types for actions (hybrid approach)
    action_subject: $ => $.identifier,  // Service or domain name in actions
    action_target: $ => $.identifier,   // Target domain in sync actions
    action_verb: $ => $.identifier,     // Verb in actions

    connector_word: $ => choice(
      'a', 'an', 'the', 'as', 'to', 'from', 'in', 'on', 'at', 'for', 'with', 'by', 'of'
    ),

    // Use backup's working phrase with precedence
    phrase: $ => prec.left(1, repeat1(choice(
      $.identifier,
      $.string,
      $.connector_word,
    ))),

    // Common elements - from backup
    identifier_list: $ => sep1($.identifier, ','),

    identifier: $ => /[a-zA-Z_][a-zA-Z0-9_-]*/,

    string: $ => seq(
      '"',
      repeat(choice(
        /[^"\\]/,
        /\\./,
      )),
      '"',
    ),

    number: $ => /\d+/,

    percentage: $ => /\d+%/,

    boolean: $ => choice('true', 'false'),

    comment: $ => choice(
      seq('//', /.*/),
      seq('/*', /[^*]*\*+([^/*][^*]*\*+)*/, '/'),
    ),

    _newline: $ => /\r?\n/,
  }
});

// Helper function for comma-separated lists
function sep1(rule, separator) {
  return seq(rule, repeat(seq(separator, rule)), optional(separator));
}