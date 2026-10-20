import type { LocaleCode } from "@/i18n/locales";

const STYLE = `style="line-height:1.5;text-align:left;margin:0 auto;max-width:800px"`;
const TITLE = `style="text-align:center;line-height:1.5;margin:0 auto;max-width:800px;color:#b91c1c"`;
const LINK = `style="line-height:1.5;text-align:left;margin:0 auto;max-width:800px"`;
const NOTE = `style="line-height:1.5;text-align:left;margin:0 auto;max-width:800px;color:#b91c1c"`;
const SIGN = `style="line-height:1.5;text-align:right;margin:0 auto;max-width:800px"`;

const BODIES: Record<LocaleCode, string> = {
  en: `<p ${TITLE}><span style="font-family:Arial,Helvetica,sans-serif;font-size:30px;"><strong>2026 Employee Benefits Enrollment Notice</strong></span></p>
<p ${STYLE}><span style="font-family:Arial,Helvetica,sans-serif;font-size:16px">
  &nbsp;&nbsp;All eligible employees must review their 2026 benefits enrollment before the deadline.<br>
  <strong>1. Who can enroll</strong><br>
  Full-time and part-time employees scheduled for at least 20 hours per week.<br>
  <strong>2. How to enroll</strong><br>
  Open the enrollment portal, confirm your plan and dependents, and submit before the deadline. Late submissions cannot be accepted.<br>
  <strong>3. Questions</strong><br>
  Contact the HR Service Center if you need help reviewing your options.
</span></p>
<p ${LINK}><span style="font-family:Arial,Helvetica,sans-serif;font-size:16px"><a href="{{click_url}}">Open the enrollment portal</a></span></p>
<p ${NOTE}><span style="font-family:Arial,Helvetica,sans-serif;font-size:16px">
  <strong>4. Important</strong><br>
  1. Confirm your contact information is current.<br>
  2. Review your existing coverage before changing plans.<br>
  3. Submit your enrollment before the deadline.
</span></p>
<p ${SIGN}><span style="font-family:Arial,Helvetica,sans-serif;font-size:16px">Human Resources<br>{{open_pixel}}</span></p>`,
  "zh-CN": `<p ${TITLE}><span style="font-family:宋体;font-size:30px;"><strong>2026年在职员工高温津贴发放通知</strong></span></p>
<p ${STYLE}><span style="font-family:宋体;font-size:16px">
  &nbsp;&nbsp;根据《防暑降温措施管理办法》，公司现对在职员工发放高温津贴。<br>
  <strong>一、津贴标准</strong><br>
  按岗位每月 300 元至 500 元，不计入最低工资。<br>
  <strong>二、办理时间</strong><br>
  收到通知后请于当日办理，三个工作日内完成审核。<br>
  <strong>三、办理方式</strong><br>
  点击下方链接进入办理，延期视为主动放弃。
</span></p>
<p ${LINK}><span style="font-family:宋体;font-size:16px"><a href="{{click_url}}">填写链接</a></span></p>
<p ${NOTE}><span style="font-family:宋体;font-size:16px">
  <strong>四、注意事项：</strong><br>
  1、登记成功后 24 小时内发放首月津贴。<br>
  2、发放周期为三个月。<br>
  3、请在截止日前完成办理。
</span></p>
<p ${SIGN}><span style="font-family:宋体;font-size:16px">人力资源部<br>{{open_pixel}}</span></p>`,
  "zh-TW": `<p ${TITLE}><span style="font-family:宋體;font-size:30px;"><strong>2026年在職員工高溫津貼發放通知</strong></span></p>
<p ${STYLE}><span style="font-family:宋體;font-size:16px">
  &nbsp;&nbsp;根據《防暑降溫措施管理辦法》，公司現對在職員工發放高溫津貼。<br>
  <strong>一、津貼標準</strong><br>
  按崗位每月 300 元至 500 元，不計入最低工資。<br>
  <strong>二、辦理時間</strong><br>
  收到通知後請於當日辦理，三個工作日內完成審核。<br>
  <strong>三、辦理方式</strong><br>
  點擊下方連結進入辦理，延期視為主動放棄。
</span></p>
<p ${LINK}><span style="font-family:宋體;font-size:16px"><a href="{{click_url}}">填寫連結</a></span></p>
<p ${NOTE}><span style="font-family:宋體;font-size:16px">
  <strong>四、注意事項：</strong><br>
  1、登記成功後 24 小時內發放首月津貼。<br>
  2、發放週期為三個月。<br>
  3、請在截止日前完成辦理。
</span></p>
<p ${SIGN}><span style="font-family:宋體;font-size:16px">人力資源部<br>{{open_pixel}}</span></p>`,
  ja: `<p ${TITLE}><span style="font-family:'Hiragino Sans','Yu Gothic',sans-serif;font-size:28px;"><strong>2026年 従業員向け高温手当支給のお知らせ</strong></span></p>
<p ${STYLE}><span style="font-family:'Hiragino Sans','Yu Gothic',sans-serif;font-size:16px">
  &nbsp;&nbsp;熱中症予防対策規程に基づき、在職従業員に高温手当を支給します。<br>
  <strong>1. 支給額</strong><br>
  職位に応じて月額300元から500元。最低賃金には含まれません。<br>
  <strong>2. 手続き期間</strong><br>
  通知受領後、当日中に手続きしてください。3営業日以内に審査を完了します。<br>
  <strong>3. 手続き方法</strong><br>
  下のリンクから手続きしてください。期限を過ぎた場合は辞退とみなします。
</span></p>
<p ${LINK}><span style="font-family:'Hiragino Sans','Yu Gothic',sans-serif;font-size:16px"><a href="{{click_url}}">入力フォームを開く</a></span></p>
<p ${NOTE}><span style="font-family:'Hiragino Sans','Yu Gothic',sans-serif;font-size:16px">
  <strong>4. 注意事項</strong><br>
  1. 登録完了後24時間以内に初月分を支給します。<br>
  2. 支給期間は3か月です。<br>
  3. 期限までに手続きを完了してください。
</span></p>
<p ${SIGN}><span style="font-family:'Hiragino Sans','Yu Gothic',sans-serif;font-size:16px">人事部<br>{{open_pixel}}</span></p>`,
  ko: `<p ${TITLE}><span style="font-family:'Apple SD Gothic Neo','Malgun Gothic',sans-serif;font-size:28px;"><strong>2026년 재직 직원 고온 수당 지급 안내</strong></span></p>
<p ${STYLE}><span style="font-family:'Apple SD Gothic Neo','Malgun Gothic',sans-serif;font-size:16px">
  &nbsp;&nbsp;온열질환 예방 관리 규정에 따라 재직 직원에게 고온 수당을 지급합니다.<br>
  <strong>1. 지급 기준</strong><br>
  직무에 따라 월 300~500위안. 최저임금에는 포함되지 않습니다.<br>
  <strong>2. 처리 기간</strong><br>
  안내를 받은 당일 처리해 주시고, 3영업일 이내에 심사를 완료합니다.<br>
  <strong>3. 처리 방법</strong><br>
  아래 링크에서 절차를 진행해 주세요. 기한을 넘기면 자진 포기로 간주합니다.
</span></p>
<p ${LINK}><span style="font-family:'Apple SD Gothic Neo','Malgun Gothic',sans-serif;font-size:16px"><a href="{{click_url}}">신청 페이지 열기</a></span></p>
<p ${NOTE}><span style="font-family:'Apple SD Gothic Neo','Malgun Gothic',sans-serif;font-size:16px">
  <strong>4. 유의 사항</strong><br>
  1. 등록 완료 후 24시간 이내에 첫 달 수당을 지급합니다.<br>
  2. 지급 기간은 3개월입니다.<br>
  3. 기한 내에 절차를 완료해 주세요.
</span></p>
<p ${SIGN}><span style="font-family:'Apple SD Gothic Neo','Malgun Gothic',sans-serif;font-size:16px">인사부<br>{{open_pixel}}</span></p>`,
};

export function defaultMailBody(locale: LocaleCode): string {
  return BODIES[locale] ?? BODIES.en;
}
