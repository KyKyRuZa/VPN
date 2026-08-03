import os
import asyncio
import logging

from aiogram import Bot, Dispatcher, types
from aiogram.filters import CommandStart
from aiogram.enums import ParseMode
from aiogram.client.default import DefaultBotProperties

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

BOT_TOKEN = os.getenv("BOT_TOKEN")
WEB_APP_URL = os.getenv("WEB_APP_URL", "https://kykyruz.cloud-ip.cc")

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


async def main() -> None:
    await dp.start_polling(bot)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except (KeyboardInterrupt, SystemExit):
        logger.info("Bot stopped")
