; Comments
(comment) @craft.comment

; === BLOCK KEYWORDS ===
"services" @craft.services-keyword
"service" @craft.services-keyword
"arch" @craft.arch-keyword  
"exposure" @craft.exposure-keyword
"domain" @craft.domain-keyword
"domains" @craft.domain-keyword
"actors" @craft.actors-keyword
"actor" @craft.actor-keyword
"use_case" @craft.flow-keyword
"when" @craft.flow-keyword

; === PROPERTY KEYWORDS ===
"domains" @craft.domains-property
"language" @craft.language-property
"data-stores" @craft.data-stores-property
"deployment" @craft.language-property
"to" @craft.to-property
"through" @craft.through-property
"of" @craft.to-property

; === SECTION KEYWORDS ===
"presentation" @craft.presentation-section
"gateway" @craft.gateway-section

; === ACTION VERBS (only those defined as keywords in grammar) ===
"asks" @craft.asks-verb
"notifies" @craft.notifies-verb
"listens" @craft.listens-verb
"returns" @craft.returns-verb

; === ACTOR TYPES ===
"user" @craft.actor-type
"system" @craft.actor-type
"service" @craft.actor-type

; === CONNECTOR WORDS ===
; Connector words in phrases should be colored as phrase words
(phrase (connector_word) @craft.phrase-word)

; Regular connector words (not in phrases)
(connector_word) @craft.connector-word

; === STRINGS - CONTEXT DEPENDENT ===
; Use case strings
(use_case_block (string) @craft.usecase-string)

; Event strings (in async actions and domain listeners)
(async_action (string) @craft.event-string)
(domain_listener (string) @craft.event-string)

; Regular strings (fallback)
(string) @craft.regular-string

; === IDENTIFIERS - GRANULAR CONTEXT SPECIFIC ===

; Component names (hybrid approach)
(component_name (identifier) @craft.component-name)

; Modifier keys and values (hybrid approach)  
(modifier_key (identifier) @craft.modifier-key)
(modifier_value (identifier) @craft.modifier-value)
(modifier_value (number) @craft.modifier-value)
(modifier_value (boolean) @craft.modifier-value)

; Action context (hybrid approach)
(action_subject (identifier) @craft.service-name)
(action_target (identifier) @craft.service-name)
(action_verb (identifier) @craft.regular-verb)

; Trigger context (hybrid approach)
(trigger_actor (identifier) @craft.actor-name)
(trigger_verb (identifier) @craft.regular-verb)

; Service names
(service_definition (identifier) @craft.service-name)

; Domain definition names
(domain_block (identifier) @craft.domain-name)
(domain_definition (identifier) @craft.domain-name)

; Exposure names
(exposure_block (identifier) @craft.exposure-name)

; Subdomain names
(subdomain (identifier) @craft.subdomain-name)

; Actor names in definitions
(actor_name (identifier) @craft.actor-definition)

; Domain list values (in domains property)
(domains_property (identifier_list (identifier) @craft.domain-list))

; Data store names (in data-stores property)
(data_stores_property (identifier_list (identifier) @craft.data-store-name))

; Exposure property values
(to_property (identifier_list (identifier) @craft.actor-name))
(through_property (identifier_list (identifier) @craft.component-name))

; Language values
(language_property (identifier) @craft.language-value)

; Phrase words - words in action phrases
(phrase (identifier) @craft.phrase-word)
(phrase (connector_word) @craft.phrase-word)

; Domain listener context
(domain_listener (identifier) @craft.service-name)

; === PUNCTUATION ===
"{" @craft.braces
"}" @craft.braces
":" @craft.colon
"," @craft.comma
">" @craft.flow-arrow

; === MODIFIER VALUES (NON-IDENTIFIERS) ===
; Numbers and booleans in modifier context handled above

; === FALLBACK FOR NUMBERS AND BOOLEANS ===
(number) @number
(boolean) @boolean