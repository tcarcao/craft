// ArchDSL.g4
grammar ArchDSL;

architecture: system* flow* ;

system: 'system' IDENT '{' context* '}' ;

context: 'bounded' 'context' IDENT '{'
    (aggregate | component | service | event | relation)*
    '}' ;

aggregate: 'aggregate' IDENT ;
component: 'component' IDENT tech? ;
service: 'service' IDENT tech? platform? ;
event: 'event' IDENT ;
relation: ('upstream'|'downstream') 'to' IDENT 'as' pattern ;

tech: 'using' TECH ;
platform: 'on' PLATFORM ;
pattern: 'acl'|'ohs'|'conformist' ;

flow: IDENT '.' IDENT '(' args? ')' ('->' target)? ;
target: IDENT '.' IDENT '(' ')' ;
args: IDENT (',' IDENT)* ;

TECH: 'go'|'java'|'python'|'nodejs'|'php' ;
PLATFORM: 'eks'|'lambda'|'sqs'|'sns'|'dynamodb'|'redis' ;
IDENT: [a-zA-Z][a-zA-Z0-9_]* ;
WS: [ \t\r\n]+ -> skip ;
LINE_COMMENT  : '//' ~[\r\n]* -> skip ;
BLOCK_COMMENT : '/*' .*? '*/' -> skip ;