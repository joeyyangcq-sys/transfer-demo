import psycopg2
from decimal import Decimal
import sys

def audit_database():
    try:
        conn = psycopg2.connect(
            host="localhost",
            port=5433,
            user="postgres",
            password="postgres",
            dbname="transfers"
        )
        cur = conn.cursor()

        print("=== 独立第三方资金审计报告 ===")
        print("1. 宏观系统资金对账 (Macro Reconciliation)")
        cur.execute("SELECT COALESCE(SUM(balance), 0) FROM accounts;")
        total_balance = cur.fetchone()[0]
        
        cur.execute("SELECT COALESCE(SUM(amount), 0) FROM deposits;")
        total_deposit = cur.fetchone()[0]

        print(f"  当前系统账户总余额: {total_balance}")
        print(f"  当前系统总入金额度: {total_deposit}")
        
        if total_balance == total_deposit:
            print("  ✅ [PASS] 宏观资金守恒！")
        else:
            print(f"  ❌ [FAIL] 资金不守恒！差额: {total_balance - total_deposit}")
            sys.exit(1)

        print("\n2. 微观单账户复式流水审计 (Micro Account Ledger Reconciliation)")
        cur.execute("SELECT id, balance FROM accounts ORDER BY id;")
        accounts = cur.fetchall()

        all_passed = True
        for acc_id, current_balance in accounts:
            cur.execute("SELECT COALESCE(SUM(amount), 0) FROM deposits WHERE account_id = %s;", (acc_id,))
            deposit = cur.fetchone()[0]

            cur.execute("SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = %s AND direction = 'credit';", (acc_id,))
            credit = cur.fetchone()[0]

            cur.execute("SELECT COALESCE(SUM(amount), 0) FROM ledger_entries WHERE account_id = %s AND direction = 'debit';", (acc_id,))
            debit = cur.fetchone()[0]

            calculated_balance = deposit + credit - debit
            
            if current_balance == calculated_balance:
                print(f"  ✅ [PASS] 账户 {acc_id}: 余额 {current_balance} = 充值 {deposit} + 转入 {credit} - 转出 {debit}")
            else:
                print(f"  ❌ [FAIL] 账户 {acc_id}: 当前余额 {current_balance} != 计算流水余额 {calculated_balance}")
                all_passed = False

        if not all_passed:
            sys.exit(1)

    except Exception as e:
        print(f"Audit script failed: {e}")
        sys.exit(1)
    finally:
        if 'conn' in locals() and conn is not None:
            cur.close()
            conn.close()

if __name__ == "__main__":
    audit_database()