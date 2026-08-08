#!/usr/bin/env python3
# Конвертирует старый config.py в config.json

import sys
import json
import importlib.util
from pathlib import Path

def convert(old_config_path):
    # Загружаем модуль config.py
    spec = importlib.util.spec_from_file_location("config", old_config_path)
    config = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(config)

    new_config = {
        "imap_server": getattr(config, 'imap_server', ''),
        "username": getattr(config, 'username', ''),
        "password": getattr(config, 'mail_pass', ''),
        "bot_token": getattr(config, 'bot_key', ''),
        "chat_id": getattr(config, 'chat_id', 0),
        "chat_id_err": getattr(config, 'chat_id_err', 0),
        "encoding": getattr(config, 'encoding', 'UTF-8'),
        "send_attach": getattr(config, 'send_attach', False),
        "proxy_url": getattr(config, 'proxy_url', ''),
        "logfile": getattr(config, 'logfile', 'mail-bot.log'),
        "loglevel": getattr(config, 'loglevel', 'info')
    }

    # Сохраняем в файл config.json рядом с конвертером
    output = Path('config.json')
    with open(output, 'w') as f:
        json.dump(new_config, f, indent=4, ensure_ascii=False)

    print(f"Конфиг успешно сконвертирован в {output}")

if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Использование: python convert_config.py <путь_к_config.py>")
        sys.exit(1)
    convert(sys.argv[1])