services {
  AccountService {
    domains: AccountManagement, BalanceTracking
    data-stores: account_db, transaction_log
    language: java
  }
  PaymentService {
    domains: PaymentProcessing, TransactionValidation
    data-stores: payment_db, fraud_detection_cache
    language: golang
  }
  NotificationService {
    domains: CustomerNotification
    data-stores: notification_queue
    language: python
  }
}

use_case "Money Transfer" {
  when Customer initiates transfer
    PaymentProcessing asks AccountManagement to verify source account
    PaymentProcessing asks TransactionValidation to check transfer limits
    TransactionValidation validates transaction rules
    PaymentProcessing asks BalanceTracking to reserve funds
    BalanceTracking notifies "Funds Reserved"

  when PaymentProcessing listens "Funds Reserved"
    PaymentProcessing asks AccountManagement to verify destination account
    PaymentProcessing executes fund transfer
    BalanceTracking updates account balances
    PaymentProcessing notifies "Transfer Completed"

  when CustomerNotification listens "Transfer Completed"
    CustomerNotification sends confirmation to both accounts
}

use_case "Account Balance Check" {
  when Customer checks balance
    AccountManagement asks BalanceTracking to get current balance
    BalanceTracking calculates available balance
    AccountManagement returns balance information
}

use_case "Suspicious Activity Detection" {
  when TransactionValidation detects suspicious pattern
    TransactionValidation asks AccountManagement to freeze account
    AccountManagement applies security hold
    TransactionValidation notifies "Account Frozen"

  when CustomerNotification listens "Account Frozen"
    CustomerNotification sends security alert to customer
    CustomerNotification notifies "bank security team"
}

use_case "Scheduled Payment Processing" {
  when CRON triggers scheduled payments
    PaymentProcessing asks AccountManagement to get scheduled payments
    PaymentProcessing processes each scheduled payment
    BalanceTracking updates balances for processed payments
    PaymentProcessing notifies "Scheduled Payments Processed"
}