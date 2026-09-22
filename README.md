# MCPGoProxy

MCP-прокси на Go для подключения LLM-клиентов к ERP-системе через HTTP-сервисы 1С.

Проект реализует Model Context Protocol (MCP) поверх существующего HTTP API ERP. MCPGoProxy получает от ERP описание доступных инструментов, преобразует его в MCP-инструменты и передаёт вызовы обратно в ERP.

## Архитектура

```text
┌──────────────────────┐
│      MCP Client      │
│                      │
│ LM Studio / Kilo /   │
│ Open WebUI / другие  │
└──────────┬───────────┘
           │
           │ MCP
           ▼
┌──────────────────────┐
│     MCPGoProxy       │
│                      │
│  Tool definitions    │
│  JSON Schema         │
│  Tool calls          │
└──────────┬───────────┘
           │
           │ HTTP
           ▼
┌──────────────────────┐
│       1C ERP         │
│                      │
│  /mcpTools           │
│  /mcpCall            │
└──────────────────────┘
```

MCPGoProxy не содержит описаний конкретных ERP-сущностей и инструментов. Эти сведения предоставляет сама ERP.

Это позволяет изменять набор доступных инструментов и их параметры на стороне 1С без необходимости изменять MCPGoProxy.

## Возможности

* подключение LLM-клиентов к ERP через MCP;
* получение динамического списка инструментов из 1С;
* динамическое формирование MCP `inputSchema`;
* поддержка вложенных объектов и массивов в параметрах инструментов;
* передача вызовов MCP-инструментов в HTTP-сервис 1С;
* получение структурированных JSON-результатов от ERP;
* работа через STDIO;
* работа через Streamable HTTP для клиентов с HTTP-подключением к MCP.

## Транспорты

MCPGoProxy поддерживает два способа подключения MCP-клиента.

### STDIO

Предназначен прежде всего для локальных приложений, которые запускают MCP-сервер как дочерний процесс.

Типичные клиенты:

* LM Studio;
* Kilo Code;
* другие MCP-клиенты с поддержкой STDIO.

Схема подключения:

```text
MCP Client
    │
    │ stdin/stdout
    ▼
MCPGoProxy.exe
    │
    │ HTTP
    ▼
1C ERP
```

Конфигурация:

```json
{
    "transport": "stdio",
    "erp_tool_url": "http://172.16.2.100/anon_mk/hs/ai/mcpTools",
    "erp_call_url": "http://172.16.2.100/anon_mk/hs/ai/mcpCall"
}
```

Если `transport` не указан, рекомендуется использовать `stdio` как режим по умолчанию для обратной совместимости.

### Streamable HTTP

Предназначен для MCP-клиентов, которые подключаются к MCP-серверу по HTTP.

Например:

* Open WebUI;
* другие клиенты с поддержкой MCP Streamable HTTP.

Схема подключения:

```text
MCP Client
    │
    │ Streamable HTTP
    ▼
MCPGoProxy
    │
    │ HTTP
    ▼
1C ERP
```

В этом режиме MCPGoProxy запускается как HTTP-сервер.

Пример конфигурации:

```json
{
    "transport": "streamable-http",
    "erp_tool_url": "http://172.16.2.100/anon_mk/hs/ai/mcpTools",
    "erp_call_url": "http://172.16.2.100/anon_mk/hs/ai/mcpCall"
}
```

Параметры HTTP-сервера MCP задаются отдельно в конфигурации прокси.

## Конфигурация

Конфигурация хранится в `config.json` рядом с исполняемым файлом.

Минимальная конфигурация:

```json
{
    "transport": "stdio",
    "erp_tool_url": "http://172.16.2.100/anon_mk/hs/ai/mcpTools",
    "erp_call_url": "http://172.16.2.100/anon_mk/hs/ai/mcpCall"
}
```

### Параметры

| Параметр       | Описание                                                         |
| -------------- | ---------------------------------------------------------------- |
| `transport`    | MCP-транспорт: `stdio` или `streamable-http`                     |
| `erp_tool_url` | HTTP-сервис 1С, возвращающий описание доступных MCP-инструментов |
| `erp_call_url` | HTTP-сервис 1С, выполняющий вызов инструмента                    |

### API-ключ ERP

Авторизация запросов к ERP выполняется через переменную окружения `ERP_API_KEY`.

Пример для Windows PowerShell:

```powershell
$env:ERP_API_KEY="ApiKey 4795A142-965D-46B2-8024-59DF7B08B855"
```

Ключ не следует хранить в `config.json` или помещать в репозиторий.

## Работа с инструментами ERP

При запуске MCPGoProxy получает от ERP описание доступных инструментов.

Упрощённо процесс выглядит следующим образом:

```text
1. MCPGoProxy → ERP /mcpTools
2. ERP → описание инструментов
3. MCPGoProxy → преобразование описания в MCP Tool
4. MCP Client → tools/list
5. MCP Client → вызов инструмента
6. MCPGoProxy → ERP /mcpCall
7. ERP → результат
8. MCPGoProxy → MCP CallToolResult
```

## Формирование JSON Schema

Описание параметров приходит из ERP в собственном формате.

Например:

```json
{
    "type": "array",
    "items": {
        "field": {
            "type": "string"
        },
        "operator": {
            "type": "string"
        },
        "value": {
            "type": "string"
        }
    }
}
```

MCPGoProxy преобразует это описание в стандартную JSON Schema, используемую MCP-клиентом.

Поддерживаются вложенные структуры, например:

```text
array
└── object
    ├── string
    └── array
        └── object
            ├── string
            ├── string
            └── string
```

Построение схемы выполняется рекурсивно.

Это важно для сложных инструментов ERP, например `query_erp`, где структура параметров может выглядеть следующим образом:

```json
{
    "filter_groups": [
        {
            "inside_logic": "AND",
            "conditions": [
                {
                    "field": "Наименование",
                    "operator": "starts_with",
                    "value": "Новик"
                }
            ]
        }
    ]
}
```

## Пример подключения к LM Studio

В конфигурации MCP-клиента LM Studio:

```json
{
    "mcpServers": {
        "GoMCPProxy": {
            "command": "E:\\DEV\\MCPGoProxy\\mcp_server.exe",
            "env": {
                "ERP_API_KEY": "ApiKey ..."
            }
        }
    }
}
```

В конфигурации MCPGoProxy при этом используется:

```json
{
    "transport": "stdio",
    "erp_tool_url": "http://172.16.2.100/anon_mk/hs/ai/mcpTools",
    "erp_call_url": "http://172.16.2.100/anon_mk/hs/ai/mcpCall"
}
```

## Пример подключения к Open WebUI

Для Open WebUI используется транспорт `streamable-http`.

MCPGoProxy запускается как HTTP-сервер, после чего MCP-сервер добавляется в Open WebUI как MCP Streamable HTTP connection.

Схема:

```text
Open WebUI
    │
    │ Streamable HTTP
    ▼
MCPGoProxy
    │
    │ HTTP
    ▼
1C
```

Конкретный URL MCP endpoint зависит от настроек HTTP-сервера MCPGoProxy.

## Разработка

Проект написан на Go.

Для сборки:

```powershell
go build
```

Для запуска:

```powershell
.\mcp_server.exe
```

Конфигурационный файл `config.json` должен находиться рядом с исполняемым файлом.

Для STDIO режима MCPGoProxy использует стандартные stdin/stdout для обмена MCP-сообщениями. Логи не должны выводиться в stdout, так как это нарушает MCP-протокол STDIO.

## Структура проекта

Основные компоненты:

```text
MCPGoProxy
│
├── MCP transport
│   ├── STDIO
│   └── Streamable HTTP
│
├── ToolSpec / ToolParam
│   └── промежуточное представление инструментов ERP
│
├── JSON Schema builder
│   ├── buildTool
│   ├── buildToolParam
│   ├── buildSchema
│   └── buildObjectSchema
│
└── ERP HTTP client
    ├── получение описания инструментов
    └── выполнение вызовов инструментов
```

## Связь с 1С

MCPGoProxy рассчитан на использование с HTTP-сервисом 1С, предоставляющим два endpoint:

```text
GET/POST .../mcpTools
```

Получение описаний инструментов.

```text
POST .../mcpCall
```

Выполнение инструмента.

Формат взаимодействия между MCPGoProxy и ERP является внутренним контрактом проекта.

Это позволяет реализовать MCP-интерфейс в 1С независимо от конкретного MCP-клиента.

## Безопасность

MCPGoProxy не должен хранить API-ключ ERP непосредственно в исходном коде или конфигурации, находящейся под контролем версий.

Рекомендуется использовать переменную окружения:

```text
ERP_API_KEY
```

При использовании Streamable HTTP необходимо учитывать, что MCP endpoint становится доступен по сети. В зависимости от сценария эксплуатации следует ограничить интерфейс `localhost` или использовать сетевую аутентификацию и HTTPS.

## Текущий статус

Проект находится в стадии активной разработки.

На текущем этапе реализованы:

* динамическое получение инструментов из ERP;
* преобразование описания инструментов в MCP;
* поддержка сложных вложенных JSON Schema;
* вызов инструментов ERP через HTTP;
* структурированные результаты MCP;
* интеграция с LM Studio;
* интеграция с Open WebUI.

## Идея проекта

Основная идея MCPGoProxy — отделить LLM от внутренней структуры ERP.

LLM не должна знать особенности HTTP-сервисов 1С и внутреннюю реализацию запросов.

Вместо этого ERP предоставляет модели семантический интерфейс:

```text
get_erp_model
       ↓
get_query_schema
       ↓
query_erp
```

MCPGoProxy выступает транспортным и протокольным слоем между этим интерфейсом и MCP-клиентом.

Таким образом, изменение модели, MCP-клиента или способа подключения не требует изменения прикладной логики ERP.
