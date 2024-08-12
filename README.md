# ArchDSL

A Domain Specific Language for describing software architecture, with support for generating various diagram types.

## Overview

ArchDSL allows you to describe software systems using a simple, readable syntax focused on:
- System boundaries and bounded contexts
- Components, services, and aggregates
- Domain-driven design concepts

## Grammar

The DSL is built using ANTLR4 and supports:
- System definitions
- Bounded context declarations  
- Aggregate, component, and service definitions

## Example

```archdsl
system OrderSystem {
   bounded context Orders {
       aggregate Order
       component OrderProcessor
       service OrderService
   }
}