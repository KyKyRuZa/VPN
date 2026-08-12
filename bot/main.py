import os
import asyncio
import logging

from aiogram import Bot, Dispatcher, types
from aiogram.filters import CommandStart, Command
from aiogram.enums import ParseMode
from aiogram.client.default import DefaultBotProperties
from aiogram.exceptions import TelegramConflictError

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

BOT_TOKEN = os.getenv("BOT_TOKEN")
WEB_APP_URL = os.getenv("WEB_APP_URL", "https://thenomoreblocks.com")

if not BOT_TOKEN:
    raise RuntimeError("BOT_TOKEN is required")

bot = Bot(token=BOT_TOKEN, default=DefaultBotProperties(parse_mode=ParseMode.HTML))
dp = Dispatcher()


@dp.message(CommandStart())
async def cmd_start(message: types.Message) -> None:
    keyboard = types.ReplyKeyboardMarkup(
        keyboard=[
            [
                types.KeyboardButton(
                    text="🚀 Открыть VPN",
                    web_app=types.WebAppInfo(url=WEB_APP_URL),
                )
            ]
        ],
        resize_keyboard=True,
        one_time_keyboard=False,
    )
    await message.answer(
        "<b>Добро пожаловать!</b>\n"
        "Нажмите кнопку ниже, чтобы открыть панель VPN и управлять подпиской.",
        reply_markup=keyboard,
    )


@dp.message(Command("id"))
async def cmd_id(message: types.Message) -> None:
    await message.answer(f"Your Telegram ID: <code>{message.from_user.id}</code>")


@dp.message(lambda m: m.web_app_data is not None)
async def web_app_data(message: types.Message) -> None:
    data = message.web_app_data.data
    await message.answer(
        f"Получены данные из мини-приложения:\n<pre>{data}</pre>",
        parse_mode=ParseMode.HTML,
    )


async def main() -> None:
    try:
        await dp.start_polling(bot)
    except TelegramConflictError:
        logger.error("TelegramConflictError: another instance is already polling with this token. Stopping.")
        raise SystemExit(0)
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
