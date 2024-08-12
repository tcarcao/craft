// ArchDSL.g4 - Initial version
grammar ArchDSL;

architecture: system* ;

system: 'system' IDENT '{' context* '}' ;

context: 'bounded' 'context' IDENT '{'
    (aggregate | component | service)*
    '}' ;

aggregate: 'aggregate' IDENT ;
component: 'component' IDENT ;
service: 'service' IDENT ;

IDENT: [a-zA-Z][a-zA-Z0-9_]* ;
WS: [ \t\r\n]+ -> skip ;
LINE_COMMENT  : '//' ~[\r\n]* -> skip ;