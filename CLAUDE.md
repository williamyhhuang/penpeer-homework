# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) 
when working with code in this repository.

始終用繁體中文說明
依照 馬斯克 第一性原理

## Project Overview
- 這是一個社群短網址專案，前後端分離
- 可以使用 docker-compose 在本機建置站台
- 詳細功能需求參考 `docs/request.pdf`

## Technology Stack
- **Golang 1.21**  for backend
- **PostgreSQL 18.0** for database access
- **gin 1.11.0** for API framework
- **Redis 7.2.7** for cache
- **TypeScript 6.0.0** for frontend
- **React 18.2.0** for frontend
- **docker docker-compose** for containerized DevOps

## Architecture
always follow docs/DDD_architecture.png

## Important Documentation

**CRITICAL**: Always read relevant documentation before 
implementing features and update documentation after making changes.

### Documentation Structure
- `README.md` - Project overview and navigation
- `CLAUDE.md` - **MUST READ** before development
- `docs/ddd_arch.png` - **MUST READ** before development

## Code Development Principles

### Version and API Management
- **ALWAYS check package versions** before writing code
- **NEVER use deprecated methods or APIs**

### Traditional Chinese Comments
- Add Traditional Chinese comments in places that are difficult to understand
- Focus on explaining "why" rather than "what"

### Rules Should be Followed
- Always git commit after implementing codes
- Always implement unit tests for codes
- Always run unit tests after implementing codes, if unit tests failed, fix it