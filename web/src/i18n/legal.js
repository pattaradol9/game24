// Bilingual legal copy for the /privacy and /terms pages.
// Each document is a list of sections; a section body item is either
// { p: 'paragraph' } or { ul: ['bullet', ...] }.
// EDIT OPERATOR before going live — it feeds names, links and dates below.

export const OPERATOR = {
  name: '24 Game',
  contactEmail: 'contact@example.com',
  siteUrl: 'https://your-domain.example',
  updated: '2026-09-07',
}

export function legalDoc(doc, lang) {
  const docs = { privacy, terms }
  return (docs[doc] && docs[doc][lang]) || docs[doc].th
}

const privacy = {
  th: [
    {
      title: 'ภาพรวม',
      body: [
        { p: `นโยบายความเป็นส่วนตัวนี้อธิบายว่า ${OPERATOR.name} (“บริการ”) เก็บข้อมูลส่วนบุคคลอะไรเมื่อคุณเล่นที่เว็บไซต์ของเรา นำไปใช้ทำอะไร และคุณมีทางเลือกอะไรบ้าง` },
        { p: 'แบบสั้น ๆ: เราเก็บให้น้อยที่สุด — ไม่มีโฆษณา ล็อกอินด้วย Google หากต้องการบันทึกความก้าวหน้าและติดกระดานผู้นำ หรือเล่นแบบผู้เล่นชั่วคราวโดยไม่ต้องมีบัญชีก็ได้ นอกจากนี้เว็บไซต์ใช้ Google Analytics เพื่อวัดการใช้งานแบบไม่ระบุตัวตน (ดูหัวข้อ “การวิเคราะห์การใช้งาน”)' },
      ],
    },
    {
      title: 'ข้อมูลที่เราเก็บ',
      body: [
        { ul: [
          'การเล่นแบบผู้เล่นชั่วคราว: ชื่อเล่นที่คุณตั้ง และรหัสเซสชันแบบสุ่มสำหรับจัดการห้อง/เซสชันเท่านั้น',
          'การล็อกอินด้วย Google: ชื่อ อีเมล ที่อยู่รูปโปรไฟล์ และรหัสบัญชี Google ของคุณ ซึ่งได้รับจาก Google ตอนคุณกดล็อกอิน',
          'ข้อมูลการเล่น: ประวัติทุกรอบที่เล่น (โหมด เลขสี่ตัวที่ได้ ผลลัพธ์ คะแนน เวลา จำนวนครั้งที่ใช้ตัวช่วย), EXP, เลเวล, สถิติและสถิติต่อเนื่องแยกตามโหมด',
          'ข้อมูลระบบ: โทเคนเซสชัน (เก็บในรูปแบบแฮชเท่านั้น), วันที่สร้างผู้เล่น และเวลาที่ใช้งานล่าสุด',
          'บนอุปกรณ์ของคุณ: เราใช้ localStorage เก็บ 3 อย่างคือ โทเคนเซสชัน การตั้งค่าเสียง และภาษาที่เลือก — เราไม่ตั้งคุกกี้ของเราเอง แต่ Google Analytics อาจตั้งคุกกี้ของตนเอง (ดูหัวข้อ “การวิเคราะห์การใช้งาน”)',
          'ข้อมูลการใช้งานเชิงสถิติ: หน้าที่เข้าชม เหตุการณ์ในเกม ประเภทอุปกรณ์/เบราว์เซอร์ และประเทศ/ภูมิภาคโดยประมาณ ซึ่ง Google Analytics เก็บในนามของเราโดยไม่ระบุตัวตน',
          'บันทึกเพื่อความปลอดภัย: ผู้ดูแลระบบอาจตรวจสอบบันทึกการกระทำในระบบภายใน (การสมัคร เปลี่ยนชื่อ การแบน) และสามารถค้นหาบัญชีด้วยอีเมลหรือชื่อเล่นได้',
        ] },
      ],
    },
    {
      title: 'สิ่งที่เรา “ไม่” เก็บ',
      body: [
        { ul: [
          'รหัสผ่าน (ระบบไม่มีรหัสผ่านเลย — ล็อกอินด้วย Google หรือโทเคนเซสชันเท่านั้น)',
          'ข้อมูลการชำระเงิน ตำแหน่งที่ตั้งแม่นยำ รายชื่อผู้ติดต่อ หรือตัวระบุการโฆษณา',
          'เราไม่มีโฆษณาในเว็บไซต์ และไม่ใช้ข้อมูลของคุณเพื่อการโฆษณา',
        ] },
      ],
    },
    {
      title: 'การวิเคราะห์การใช้งาน (Google Analytics)',
      body: [
        { p: 'เว็บไซต์ใช้ Google Analytics 4 ซึ่งเป็นบริการวิเคราะห์การใช้งานเว็บของ Google เพื่อช่วยให้เราเข้าใจว่าผู้เล่นใช้งานเว็บไซต์อย่างไร (หน้าที่เข้าชม โหมดที่เล่น เหตุการณ์ในเกม อุปกรณ์/เบราว์เซอร์ และประเทศ/ภูมิภาคโดยประมาณ) เพื่อนำไปปรับปรุงบริการ' },
        { ul: [
          'Google Analytics อาจตั้งคุกกี้ “_ga” เพื่อแยกผู้ใช้และเซสชันด้วยรหัสสุ่ม (Client ID) ซึ่งไม่มีชื่อ อีเมล หรือข้อมูลบัญชีของคุณอยู่ในนั้น',
          'Google Analytics ไม่เก็บหรือบันทึกที่อยู่ IP แบบเต็มของคุณ',
          'ข้อมูลวิเคราะห์เหล่านี้ใช้เพื่อสถิติและการปรับปรุงบริการเท่านั้น เราไม่ส่งข้อมูลส่วนบุคคลของคุณ (ชื่อ อีเมล) ให้ Google Analytics และไม่ใช้ข้อมูลนี้เพื่อการโฆษณา',
        ] },
        { p: 'คุณปฏิเสธการเก็บข้อมูลของ Google Analytics ได้ เช่น การติดตั้งส่วนเสริม Opt-out ของเบราว์เซอร์ (https://tools.google.com/dlpage/gaoptout) หรือการบล็อกคุกกี้จากเมนูตั้งค่าเบราว์เซอร์ รายละเอียดการประมวลผลข้อมูลของ Google ดูได้ที่ https://policies.google.com/privacy' },
      ],
    },
    {
      title: 'วิธีที่เราใช้ข้อมูล',
      body: [
        { ul: [
          'สร้างโปรไฟล์ผู้เล่น บันทึก EXP/เลเวล และรักษาสถานะการล็อกอิน',
          'แสดงชื่อเล่นและรูปโปรไฟล์บนกระดานผู้นำสาธารณะ (เฉพาะผู้เล่นที่ล็อกอิน — ผู้เล่นชั่วคราวไม่ปรากฏบนกระดาน)',
          'ให้บริการห้องแข่งกับเพื่อนและบันทึกผลการแข่ง',
          'ตรวจจับการใช้งานที่ไม่เหมาะสม บังคับใช้การเล่นอย่างเป็นธรรม และแบนบัญชีที่ละเมิดข้อกำหนดการใช้บริการ',
          'ดูแลระบบ (สำรองข้อมูล จัดการฐานข้อมูล) ผ่านหน้าผู้ดูแลที่จำกัดสิทธิ์เฉพาะผู้ดูแล',
        ] },
      ],
    },
    {
      title: 'ข้อมูลที่เปิดเผยต่อสาธารณะ',
      body: [
        { p: 'หากคุณล็อกอินด้วย Google ชื่อเล่นและรูปโปรไฟล์ของคุณอาจแสดงบนกระดานผู้นำสาธารณะและในห้องแข่ง อีเมลของคุณจะไม่ถูกแสดงต่อสาธารณะเด็ดขาด' },
      ],
    },
    {
      title: 'ผู้ให้บริการจากภายนอก',
      body: [
        { ul: [
          'Google Identity Services — ใช้สำหรับปุ่ม “ล็อกอินด้วย Google” ซึ่ง Google จะประมวลผลข้อมูลตามนโยบายความเป็นส่วนตัวของ Google เอง',
          'Google Fonts — เว็บไซต์โหลดฟอนต์ Kanit จากเซิร์ฟเวอร์ฟอนต์ของ Google',
          'CDN รูปภาพของ Google — รูปโปรไฟล์โหลดจาก googleusercontent.com',
          'Google Analytics 4 — บริการวิเคราะห์การใช้งานของ Google ซึ่งประมวลผลข้อมูลตามนโยบายของ Google (https://policies.google.com/technologies/partner-sites)',
        ] },
        { p: 'เราไม่ขายหรือแบ่งปันข้อมูลส่วนบุคคลของคุณให้ผู้ใดอื่น' },
      ],
    },
    {
      title: 'การจัดเก็บและความปลอดภัย',
      body: [
        { p: 'ข้อมูลถูกเก็บในฐานข้อมูลบนเซิร์ฟเวอร์ของเรา โดยฟิลด์ข้อมูลส่วนบุคคล (อีเมล รหัสบัญชี Google ชื่อเล่น URL รูปโปรไฟล์ และโทเคนเซสชัน) ถูกเข้ารหัสขณะจัดเก็บด้วย AES-256-GCM และการค้นหาใช้ค่าแฮชแบบเติมเกลือแทนการเก็บข้อมูลดิบ' },
        { p: 'อย่างไรก็ตาม ไม่มีวิธีส่งข้อมูลหรือจัดเก็บข้อมูลใดที่ปลอดภัย 100% เราจึงไม่สามารถรับประกันความปลอดภัยสัมบูรณ์ได้' },
      ],
    },
    {
      title: 'การเก็บรักษาและการลบข้อมูล',
      body: [
        { p: 'เราเก็บข้อมูลของคุณตราบใดที่บันทึกผู้เล่นของคุณยังอยู่ในระบบ ผู้เล่นชั่วคราวสามารถล้างข้อมูลฝั่งอุปกรณ์ได้ด้วยการเคลียร์ข้อมูลเว็บไซต์ในเบราว์เซอร์' },
        { p: 'คุณขอให้เราลบบัญชีของคุณได้ทางอีเมลด้านล่าง การลบจะลบโปรไฟล์ สถิติ และประวัติการเล่นทั้งหมดอย่างถาวรและไม่สามารถกู้คืนได้ นอกจากนี้เราอาจลบบันทึกผู้เล่นชั่วคราวที่ไม่มีการใช้งานเป็นเวลานานออกจากระบบ' },
      ],
    },
    {
      title: 'สิทธิและทางเลือกของคุณ',
      body: [
        { ul: [
          'เลือกเล่นแบบผู้เล่นชั่วคราว — เราจะไม่เก็บอะไรมากไปกว่าชื่อเล่น',
          'เปลี่ยนชื่อเล่นได้ตลอดเวลาจากเมนูโปรไฟล์',
          'ออกจากระบบเพื่อล้างเซสชันในเครื่องของคุณ',
          'ขอเข้าถึง ขอสำเนา หรือขอลบข้อมูลของคุณ',
          'ปฏิเสธหรือบล็อก Google Analytics ได้ตลอดเวลา (ดูหัวข้อ “การวิเคราะห์การใช้งาน”)',
          'ถอนความยินยอมได้ตลอดเวลาโดยหยุดใช้บริการ',
        ] },
        { p: 'ขึ้นอยู่กับกฎหมายในประเทศที่คุณอาศัย (เช่น พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล พ.ศ. 2562 (PDPA) ของไทย หรือ GDPR ของยุโรป) คุณอาจมีสิทธิ์ตามกฎหมายเพิ่มเติม ติดต่อเราได้ที่อีเมลด้านล่าง' },
      ],
    },
    {
      title: 'เด็กและเยาวชน',
      body: [
        { p: 'บริการนี้ไม่ได้มุ่งเป้าไปที่เด็กอายุต่ำกว่า 13 ปี และเราไม่ตั้งใจเก็บข้อมูลส่วนบุคคลของเด็กในวัยนี้ หากคุณเชื่อว่ามีเด็กอายุต่ำกว่า 13 ปีส่งข้อมูลให้เรา กรุณาติดต่อเราเพื่อให้เราลบข้อมูลนั้น' },
      ],
    },
    {
      title: 'การเปลี่ยนแปลงนโยบาย',
      body: [
        { p: 'เราอาจปรับปรุงนโยบายนี้เป็นครั้งคราว โดยวันที่ “อัปเดตล่าสุด” ด้านบนจะเปลี่ยนตาม และการเปลี่ยนแปลงที่สำคัญจะแจ้งบนเว็บไซต์' },
      ],
    },
    {
      title: 'ติดต่อเรา',
      body: [
        { p: `หากมีคำถามเกี่ยวกับนโยบายนี้ ติดต่อได้ที่ ${OPERATOR.contactEmail}` },
      ],
    },
  ],
  en: [
    {
      title: 'Overview',
      body: [
        { p: `This Privacy Policy explains what personal data ${OPERATOR.name} (“the Service”) collects when you play on our website, how we use it, and the choices you have.` },
        { p: 'The short version: we collect as little as possible — no ads. Sign in with Google if you want saved progress and the leaderboard, or play as a guest without an account. The site also uses Google Analytics to measure anonymous, aggregate usage (see “Analytics”).' },
      ],
    },
    {
      title: 'Data we collect',
      body: [
        { ul: [
          'Guest play: the nickname you type, plus a random session identifier for session/room management. Nothing else.',
          'Google sign-in: your Google name, email address, profile picture URL and Google account ID, received from Google when you sign in.',
          'Gameplay data: every round you play (game mode, the four numbers dealt, result, points, time taken, hints used), your EXP, level, streaks and per-mode statistics.',
          'Service data: a session token (stored only as a hash), the date your player record was created, and when you were last active.',
          'On your device: we use browser localStorage for three things only — your session token, sound preference and chosen language. We set no cookies of our own, but Google Analytics may set its own cookies (see “Analytics” below).',
          'Usage analytics: pages viewed, in-game events, device/browser type and approximate country/region, collected by Google Analytics on our behalf and not linked to your identity.',
          'Security records: administrators may review an internal audit trail of account actions (creation, renames, bans) and can look up accounts by email or nickname.',
        ] },
      ],
    },
    {
      title: 'What we do NOT collect',
      body: [
        { ul: [
          'Passwords (the Service has none — sign-in uses Google or a session token).',
          'Payment data, precise location, contacts, or advertising identifiers.',
          'We run no ads on the site and do not use your data for advertising.',
        ] },
      ],
    },
    {
      title: 'Analytics (Google Analytics)',
      body: [
        { p: 'The site uses Google Analytics 4, Google’s web analytics service, to understand how players use the site (pages viewed, game modes, in-game events, device/browser type and approximate country/region) so we can improve the Service.' },
        { ul: [
          'Google Analytics may set the “_ga” cookie to distinguish users and sessions via a random Client ID, which contains no name, email or account data.',
          'Google Analytics does not store or log your full IP address.',
          'This analytics data is used for statistics and improving the Service only. We never send your personal data (name, email) to Google Analytics and never use this data for advertising.',
        ] },
        { p: 'You can opt out of Google Analytics, for example by installing the browser opt-out add-on (https://tools.google.com/dlpage/gaoptout) or blocking cookies in your browser settings. Details of how Google processes this data: https://policies.google.com/privacy' },
      ],
    },
    {
      title: 'How we use data',
      body: [
        { ul: [
          'Create your player profile, save EXP and level, and keep you signed in.',
          'Display your nickname and profile picture on public leaderboards (signed-in players only — guests never appear).',
          'Operate friend rooms (multiplayer) and record match results.',
          'Detect abuse, enforce fair play, and ban accounts that break the Terms of Service.',
          'Administer the service (backups, database management) through a restricted admin portal.',
        ] },
      ],
    },
    {
      title: 'Public information',
      body: [
        { p: 'If you sign in with Google, your chosen nickname and Google profile picture may be shown publicly on leaderboards and in friend rooms. Your email address is never shown publicly.' },
      ],
    },
    {
      title: 'Third-party services',
      body: [
        { ul: [
          'Google Identity Services — powers “Sign in with Google”. Google processes that data under its own privacy policy.',
          'Google Fonts — the site loads the Kanit typeface from Google font servers.',
          "Google's avatar CDN — profile pictures load from googleusercontent.com.",
          'Google Analytics 4 — Google’s web analytics service, which processes that data under Google’s own policies (https://policies.google.com/technologies/partner-sites).',
        ] },
        { p: 'We do not sell or share your personal data with anyone else.' },
      ],
    },
    {
      title: 'Storage & security',
      body: [
        { p: 'Data is stored in a database on our server. Personal fields (email, Google account ID, nickname, avatar URL and session tokens) are encrypted at rest with AES-256-GCM, and lookups use salted hashes instead of the raw values.' },
        { p: 'That said, no method of transmission or storage is 100% secure, and we cannot guarantee absolute security.' },
      ],
    },
    {
      title: 'Retention & deletion',
      body: [
        { p: 'We keep your data for as long as your player record exists. Guests can clear the device-side data by clearing this site’s data in their browser.' },
        { p: `You can ask us to delete your account via the email below. Deletion permanently removes your profile, statistics and round history, and cannot be undone. We may also remove long-inactive guest records.` },
      ],
    },
    {
      title: 'Your choices & rights',
      body: [
        { ul: [
          'Play as a guest — we collect nothing beyond your nickname.',
          'Change your nickname at any time from the profile menu.',
          'Sign out to clear the session stored on your device.',
          'Request access to, a copy of, or deletion of your data.',
          'Opt out of or block Google Analytics at any time (see “Analytics”).',
          'Withdraw consent at any time by stopping your use of the Service.',
        ] },
        { p: 'Depending on where you live (for example under Thailand’s PDPA or the EU/EEA GDPR) you may have additional statutory rights. Contact us at the email below.' },
      ],
    },
    {
      title: 'Children',
      body: [
        { p: 'The Service is not directed at children under 13 and we do not knowingly collect their personal data. If you believe a child under 13 has provided us data, contact us and we will delete it.' },
      ],
    },
    {
      title: 'Changes to this policy',
      body: [
        { p: 'We may update this policy from time to time. The “Last updated” date above will change, and significant changes will be announced on the website.' },
      ],
    },
    {
      title: 'Contact',
      body: [
        { p: `Questions about this policy? Contact us at ${OPERATOR.contactEmail}.` },
      ],
    },
  ],
}

const terms = {
  th: [
    {
      title: 'การยอมรับข้อกำหนด',
      body: [
        { p: `การเข้าใช้หรือเล่น ${OPERATOR.name} ถือว่าคุณยอมรับข้อกำหนดการใช้บริการนี้และนโยบายความเป็นส่วนตัวของเรา หากไม่ยอมรับ กรุณาอย่าใช้บริการ` },
      ],
    },
    {
      title: 'บริการ',
      body: [
        { p: 'บริการนี้เป็นเกมปริศนาคณิตศาสตร์บนเบราว์เซอร์ (โหมดเล่นคนเดียว ห้องแข่งกับเพื่อน ระบบ EXP และกระดานผู้นำ) ให้บริการฟรี “ตามสภาพ” โดยไม่มีค่าใช้จ่าย' },
      ],
    },
    {
      title: 'การบริจาค',
      body: [
        { p: 'การสนับสนุนผ่าน QR PromptPay ที่แสดงบนเว็บไซต์เป็น “การบริจาคให้โดยเสียมูลค่า” (unconditional gift) โดยสมัครใจ ไม่ใช่การซื้อขายสินค้า การว่าจ้าง การชำระค่าบริการ หรือการลงทุน และไม่มีสิ่งตอบแทนใด ๆ ทั้งสิ้น ไม่ว่าโดยทั้งหมดหรือบางส่วน' },
        { ul: [
          'ผู้บริจาคจะไม่ได้รับสินค้า บริการ ไอเทม ส่วนลด สิทธิพิเศษ สถานะ หรือข้อได้เปรียบในเกมใด ๆ ตอบแทนการบริจาค',
          'การบริจาคไม่ก่อให้เกิดสิทธิเรียกร้องใด ๆ ต่อบริการ และไม่มีผลผูกพันให้ผู้ให้บริการต้องให้สิ่งใดตอบแทน ทั้งในอดีต ปัจจุบัน หรืออนาคต',
          'การบริจาคเป็นการให้ขาดและไม่สามารถขอคืนได้ (non-refundable) ผู้บริจาคควรตรวจสอบจำนวนเงินและบัญชีปลายทางก่อนโอนทุกครั้ง',
          'เราไม่เก็บข้อมูลการชำระเงินหรือข้อมูลส่วนบุคคลใด ๆ จากการบริจาค และการบริจาคไม่มีเงื่อนไขผูกกับบัญชีผู้เล่น',
          'การบริจาคใช้ช่องทาง PromptPay รับเงินเข้าบัญชีส่วนบุคคลของผู้ให้บริการเท่านั้น เราไม่มีตัวแทนหรือช่องทางรับเงินอื่นใด',
        ] },
      ],
    },
    {
      title: 'บัญชีผู้เล่น',
      body: [
        { ul: [
          'คุณสามารถเล่นแบบผู้เล่นชั่วคราว (ใช้แค่ชื่อเล่น ไม่บันทึกสถิติ) หรือล็อกอินด้วย Google',
          'บัญชีผู้เล่นหนึ่งบัญชีต่อหนึ่งคน คุณรับผิดชอบกิจกรรมที่เกิดขึ้นภายใต้เซสชันของคุณ และควรเก็บโทเคนเซสชันของคุณให้ปลอดภัย',
          'ห้ามตั้งชื่อเล่นที่ไม่เหมาะสม หยาบคาย ลอกเลียนแบบผู้อื่น หรือละเมิดสิทธิ์ของผู้อื่น',
          'เราอาจเปลี่ยนชื่อเล่นของบัญชีที่ใช้ชื่อไม่เหมาะสม',
        ] },
      ],
    },
    {
      title: 'การเล่นอย่างเป็นธรรมและพฤติกรรมที่ยอมรับได้',
      body: [
        { p: 'เมื่อใช้บริการ คุณตกลงว่าจะไม่:', },
        { ul: [
          'โกงหรือใช้ระบบอัตโนมัติ บอท สคริปต์ หรือช่องโหว่ของระบบเพื่อได้เปรียบบนกระดานผู้นำ',
          'ก่อกวน คุกคาม หรือแสดงเนื้อหาที่ไม่เหมาะสมต่อผู้เล่นอื่น รวมถึงในห้องแข่ง',
          'ดึงข้อมูลขนานใหญ่ (scraping) ยิงคำขอรบกวนระบบ หรือพยายามเข้าถึงหน้าผู้ดูแลหรือข้อมูลของผู้เล่นอื่น',
          'ใช้บริการเพื่อวัตถุประสงค์ที่ผิดกฎหมาย',
        ] },
      ],
    },
    {
      title: 'การกำกับดูแลและการบังคับใช้',
      body: [
        { p: 'ผู้ดูแลระบบมีสิทธิ์เปลี่ยนชื่อเล่น ระงับ แบน หรือลบบัญชีอย่างถาวร (รวมถึงสถิติและประวัติทั้งหมด) สำหรับบัญชีที่ละเมิดข้อกำหนดนี้ โดยไม่ต้องแจ้งล่วงหน้า' },
        { p: 'บัญชีที่ถูกแบนจะเสียสิทธิ์เข้าถึงความก้าวหน้าที่บันทึกไว้และกระดานผู้นำ' },
      ],
    },
    {
      title: 'เนื้อหาของคุณ',
      body: [
        { p: 'ชื่อเล่นและรูปโปรไฟล์ของคุณอาจถูกแสดงบนกระดานผู้นำและในห้องแข่ง คุณเป็นเจ้าของเนื้อหาเหล่านั้น แต่ให้สิทธิ์แก่เราในการจัดเก็บและแสดงผลเป็นส่วนหนึ่งของบริการ คุณรับผิดชอบต่อชื่อเล่นที่คุณเลือกใช้' },
      ],
    },
    {
      title: 'ทรัพย์สินทางปัญญา',
      body: [
        { p: `โค้ด ดีไซน์ และตราสินค้าของ ${OPERATOR.name} เป็นทรัพย์สินของผู้ให้บริการ สิทธิ์ในการใช้งานที่มอบให้คุณเป็นการใช้งานส่วนบุคคล ไม่มีค่าตอบแทน และไม่ถ่ายโอนกรรมสิทธิ์` },
      ],
    },
    {
      title: 'การปฏิเสธการรับประกัน',
      body: [
        { p: 'บริการได้รับการจัดให้ “ตามสภาพ” และ “ตามที่มี” โดยไม่มีการรับประกันใด ๆ ไม่ว่าโดยชัดแจ้งหรือโดยปริยาย รวมถึงการรับประกันว่าบริการจะไม่หยุดชะงัก ปราศจากข้อผิดพลาด หรือเหมาะสมกับวัตถุประสงค์ใด' },
      ],
    },
    {
      title: 'การจำกัดความรับผิด',
      body: [
        { p: 'ในขอบเขตสูงสุดที่กฎหมายอนุญาต ผู้ให้บริการจะไม่รับผิดต่อความเสียหายทางอ้อม โดยบังเอิญ พิเศษ หรือตามผลลัพธ์ รวมถึงการสูญเสียข้อมูลหรือกำไร ที่เกิดจากการใช้บริการ' },
        { p: 'ความก้าวหน้า EXP และสถิติในเกมอาจสูญหายจากข้อผิดพลาด การบำรุงรักษา หรือการรีเซ็ตระบบ บริการนี้เป็นเกมเพื่อความบันเทิงโดยไม่มีมูลค่าทางการเงิน' },
      ],
    },
    {
      title: 'การเปลี่ยนแปลงและความพร้อมใช้งาน',
      body: [
        { p: 'เราอาจเปลี่ยนแปลง ระงับ หรือยุติส่วนใดส่วนหนึ่งของบริการ (รวมถึงการรีเซ็ตกระดานผู้นำ) ได้ตลอดเวลา' },
        { p: 'เราอาจปรับปรุงข้อกำหนดนี้เป็นครั้งคราว การใช้บริการต่อหลังการเปลี่ยนแปลงถือว่าคุณยอมรับข้อกำหนดฉบับปรับปรุง' },
      ],
    },
    {
      title: 'กฎหมายที่ใช้บังคับ',
      body: [
        { p: 'ข้อกำหนดนี้อยู่ภายใต้กฎหมายของราชอาณาจักรไทย' },
      ],
    },
    {
      title: 'ติดต่อเรา',
      body: [
        { p: `หากมีคำถามเกี่ยวกับข้อกำหนดนี้ ติดต่อได้ที่ ${OPERATOR.contactEmail}` },
      ],
    },
  ],
  en: [
    {
      title: 'Acceptance',
      body: [
        { p: `By accessing or playing ${OPERATOR.name} you agree to these Terms of Service and to our Privacy Policy. If you do not agree, please do not use the Service.` },
      ],
    },
    {
      title: 'The Service',
      body: [
        { p: 'The Service is a free browser-based math puzzle game (solo play, friend rooms, an EXP system and leaderboards), provided “as is” at no charge.' },
      ],
    },
    {
      title: 'Donations',
      body: [
        { p: 'Support sent via the PromptPay QR shown on the website is a voluntary, unconditional gift. It is not a sale of goods, a hire of work, a payment for services, or an investment, and no consideration of any kind is provided in return, in whole or in part.' },
        { ul: [
          'Donors receive no goods, services, items, discounts, perks, status, or in-game advantages in exchange for a donation.',
          'A donation creates no claim or entitlement to the Service, and does not obligate the operator to provide anything in return, past, present or future.',
          'Donations are final and non-refundable. Please verify the amount and the destination account before every transfer.',
          'We collect no payment data and no personal data from donations, and a donation is not tied to any player account.',
          'Donations are accepted only through PromptPay into the operator’s personal bank account. We have no agents or any other payment channel.',
        ] },
      ],
    },
    {
      title: 'Player accounts',
      body: [
        { ul: [
          'You may play as a guest (nickname only, no saved stats) or sign in with Google.',
          'One player account per person. You are responsible for activity under your session and should keep your session token private.',
          'Do not choose a nickname that is offensive, impersonates others, or infringes anyone’s rights.',
          'We may rename accounts that use inappropriate nicknames.',
        ] },
      ],
    },
    {
      title: 'Fair play & acceptable use',
      body: [
        { p: 'When using the Service, you agree not to:' },
        { ul: [
          'Cheat: no automation, bots, scripts, or exploiting bugs for leaderboard advantage.',
          'Harass other players or post offensive content, including in friend rooms.',
          'Scrape data, flood the API with requests, or attempt to access admin functions or other players’ data.',
          'Use the Service for any unlawful purpose.',
        ] },
      ],
    },
    {
      title: 'Moderation & enforcement',
      body: [
        { p: 'Administrators may rename, suspend, ban, or permanently delete accounts (including all statistics and history) for violations of these Terms, with or without prior notice.' },
        { p: 'A banned account loses access to its saved progress and to the leaderboards.' },
      ],
    },
    {
      title: 'Your content',
      body: [
        { p: 'Your nickname and profile picture may be displayed on leaderboards and in friend rooms. You keep ownership of them, but grant us the right to store and display them as part of the Service. You are responsible for the nickname you choose.' },
      ],
    },
    {
      title: 'Intellectual property',
      body: [
        { p: `The ${OPERATOR.name} code, design and branding belong to the operator. The rights granted to you are for personal, non-commercial use and do not transfer ownership.` },
      ],
    },
    {
      title: 'Disclaimer',
      body: [
        { p: 'The Service is provided “as is” and “as available”, without warranties of any kind, express or implied, including warranties that it will be uninterrupted, error-free, or fit for any particular purpose.' },
      ],
    },
    {
      title: 'Limitation of liability',
      body: [
        { p: 'To the maximum extent permitted by law, the operator is not liable for indirect, incidental, special or consequential damages, including loss of data or profits, arising from your use of the Service.' },
        { p: 'In-game progress, EXP and statistics may be lost due to bugs, maintenance or resets. The Service is a game for entertainment with no monetary value.' },
      ],
    },
    {
      title: 'Changes & availability',
      body: [
        { p: 'We may change, suspend or discontinue any part of the Service (including resetting leaderboards) at any time.' },
        { p: 'We may update these Terms from time to time. Continued use of the Service after a change means you accept the updated Terms.' },
      ],
    },
    {
      title: 'Governing law',
      body: [
        { p: 'These Terms are governed by the laws of the Kingdom of Thailand.' },
      ],
    },
    {
      title: 'Contact',
      body: [
        { p: `Questions about these Terms? Contact us at ${OPERATOR.contactEmail}.` },
      ],
    },
  ],
}
