const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const app=fs.readFileSync('goassets/template/_task_mecca/framework/web/app.js','utf8');
const css=fs.readFileSync('goassets/template/_task_mecca/framework/web/style.css','utf8');
test('Telegram connection buttons adopt shared action-btn styling',()=>{
 assert.match(app,/class="action-btn telegram-message-action/);
 assert.doesNotMatch(app,/class="secondary-btn" data-telegram-test-project/);
});
test('Telegram bot token and chat connection are distinct',()=>{
 assert.match(app,/telegramBotRegisteredCount/);
 assert.match(app,/telegramBotChatCount/);
 assert.match(app,/telegramBotSharedCount/);
 assert.match(app,/telegramBotNotRegistered/);
});
test('All-project bot token replacement is explicit and requires warning',()=>{
 assert.match(app,/전체 프로젝트 알림용 봇 토큰 등록/);
 assert.match(app,/전체 프로젝트 알림용 봇 토큰 변경/);
 assert.match(app,/telegramSharedReplacementWarning/);
 assert.match(app,/id="sharedTelegramToken" type="password"/);
});
